package googleapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

var errAdsMutationRequest = errors.New("invalid Google Ads mutation request")

// PrepareAdsMutation restricts endpoints and owns validation/atomicity flags.
// The request remains in Google's REST format, including temporary resource IDs.
func PrepareAdsMutation(service string, body []byte, apply bool) (map[string]json.RawMessage, error) {
	switch service {
	case "googleAds", "campaigns", "campaignBudgets", "campaignCriteria", "adGroups", "adGroupCriteria", "adGroupAds", "ads", "experiments", "experimentArms", "customConversionGoals", "conversionGoalCampaignConfigs":
	default:

		return nil, fmt.Errorf("%w: unsupported service %q", errAdsMutationRequest, service)
	}

	var request map[string]json.RawMessage

	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("decode mutation JSON: %w", err)
	}

	if request == nil {
		return nil, fmt.Errorf("%w: must be a JSON object", errAdsMutationRequest)
	}

	operationsKey := "operations"

	if service == "googleAds" {
		operationsKey = "mutateOperations"
	}

	for key := range request {
		if key != operationsKey && key != "validateOnly" && key != "partialFailure" && key != "responseContentType" {
			return nil, fmt.Errorf("%w: unsupported field %q", errAdsMutationRequest, key)
		}
	}

	var operations []json.RawMessage

	if err := json.Unmarshal(request[operationsKey], &operations); err != nil || len(operations) == 0 {
		return nil, fmt.Errorf("%w: %s must be a nonempty array", errAdsMutationRequest, operationsKey)
	}

	if len(operations) > 10000 {
		return nil, fmt.Errorf("%w: exceeds 10000 operations", errAdsMutationRequest)
	}

	for _, operation := range operations {
		var fields map[string]json.RawMessage

		if err := json.Unmarshal(operation, &fields); err != nil || len(fields) == 0 {
			return nil, fmt.Errorf("%w: each operation must be a nonempty object", errAdsMutationRequest)
		}

	}

	// Never let file content defeat the command's default validation-only mode.
	request["validateOnly"] = json.RawMessage("true")

	if apply {
		request["validateOnly"] = json.RawMessage("false")
	}

	request["partialFailure"] = json.RawMessage("false")

	return request, nil
}

// Mutate submits one atomic request. Applied writes are not automatically retried:
// a timeout can have committed remotely and must be reconciled before retrying.
func (c *AdsClient) Mutate(ctx context.Context, customerID, service string, body []byte, apply bool) (json.RawMessage, error) {
	id, err := NormalizeAdsCustomerID(customerID)
	if err != nil {
		return nil, err
	}

	request, err := PrepareAdsMutation(service, body, apply)
	if err != nil {
		return nil, err
	}

	if ReadOnly(ctx) {
		return nil, fmt.Errorf("%w: Google Ads mutation", ErrReadOnly)
	}

	var response json.RawMessage

	if apply {
		ctx = WithoutRetries(ctx)
	}

	err = c.doJSON(ctx, http.MethodPost, c.endpoint("customers/"+id+"/"+service+":mutate"), request, &response)

	return response, err
}

// ScheduleExperiment starts asynchronous experiment setup. A successful response
// is an operation receipt, not proof that the experiment is serving.
func (c *AdsClient) ScheduleExperiment(ctx context.Context, customerID, experimentID string, apply bool) (json.RawMessage, error) {
	id, err := NormalizeAdsCustomerID(customerID)
	if err != nil {
		return nil, err
	}

	experiment, err := NormalizeAdsCustomerID(experimentID)
	if err != nil {
		return nil, err
	}

	if ReadOnly(ctx) {
		return nil, fmt.Errorf("%w: schedule Google Ads experiment", ErrReadOnly)
	}

	if apply {
		ctx = WithoutRetries(ctx)
	}

	var response json.RawMessage
	err = c.doJSON(ctx, http.MethodPost, c.endpoint("customers/"+id+"/experiments/"+experiment+":scheduleExperiment"), map[string]bool{"validateOnly": !apply}, &response)

	return response, err
}
