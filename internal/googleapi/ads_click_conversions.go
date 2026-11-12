package googleapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var errAdsClickUpload = errors.New("invalid Google Ads click conversion upload")

// Fields a click conversion may carry. Anything else (for example user identifiers)
// is refused, so an upload can only ever send a click id, the action, time, value
// and the order id used for deduplication.
var allowedClickConversionFields = map[string]bool{
	"gclid": true, "gbraid": true, "wbraid": true,
	"conversionAction": true, "conversionDateTime": true,
	"conversionValue": true, "currencyCode": true, "orderId": true,
}

var conversionActionResource = regexp.MustCompile(`^customers/\d+/conversionActions/\d+$`)

// PrepareClickConversionUpload checks an UploadClickConversions request and owns its
// flags: validation only unless apply, and partial failure on (Google requires it).
func PrepareClickConversionUpload(customerID string, body []byte, apply bool) (map[string]json.RawMessage, error) {
	var request map[string]json.RawMessage
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("decode upload JSON: %w", err)
	}

	if request == nil {
		return nil, fmt.Errorf("%w: must be a JSON object", errAdsClickUpload)
	}

	for key := range request {
		if key != "conversions" && key != "validateOnly" && key != "partialFailure" {
			return nil, fmt.Errorf("%w: unsupported field %q", errAdsClickUpload, key)
		}
	}

	var conversions []map[string]json.RawMessage
	if err := json.Unmarshal(request["conversions"], &conversions); err != nil || len(conversions) == 0 {
		return nil, fmt.Errorf("%w: conversions must be a nonempty array", errAdsClickUpload)
	}

	if len(conversions) > 2000 {
		return nil, fmt.Errorf("%w: exceeds 2000 conversions", errAdsClickUpload)
	}

	for index, conversion := range conversions {
		clickIDs := 0

		for key := range conversion {
			if !allowedClickConversionFields[key] {
				return nil, fmt.Errorf("%w: conversion %d has unsupported field %q", errAdsClickUpload, index, key)
			}

			if key == "gclid" || key == "gbraid" || key == "wbraid" {
				clickIDs++
			}
		}

		if clickIDs != 1 {
			return nil, fmt.Errorf("%w: conversion %d needs exactly one of gclid, gbraid or wbraid", errAdsClickUpload, index)
		}

		var action, at, order string
		if json.Unmarshal(conversion["conversionAction"], &action) != nil || !conversionActionResource.MatchString(action) {
			return nil, fmt.Errorf("%w: conversion %d needs conversionAction customers/<id>/conversionActions/<id>", errAdsClickUpload, index)
		}

		if !strings.HasPrefix(action, "customers/"+customerID+"/") {
			return nil, fmt.Errorf("%w: conversion %d targets another customer", errAdsClickUpload, index)
		}

		if json.Unmarshal(conversion["conversionDateTime"], &at) != nil || at == "" {
			return nil, fmt.Errorf("%w: conversion %d needs conversionDateTime", errAdsClickUpload, index)
		}

		if json.Unmarshal(conversion["orderId"], &order) != nil || order == "" {
			return nil, fmt.Errorf("%w: conversion %d needs orderId for deduplication", errAdsClickUpload, index)
		}
	}

	request["validateOnly"] = json.RawMessage("true")
	if apply {
		request["validateOnly"] = json.RawMessage("false")
	}
	request["partialFailure"] = json.RawMessage("true")

	return request, nil
}

// UploadClickConversions sends offline click conversions. Applied uploads are not
// retried automatically; Google drops a repeated orderId for the same action.
func (c *AdsClient) UploadClickConversions(ctx context.Context, customerID string, body []byte, apply bool) (json.RawMessage, error) {
	id, err := NormalizeAdsCustomerID(customerID)
	if err != nil {
		return nil, err
	}

	request, err := PrepareClickConversionUpload(id, body, apply)
	if err != nil {
		return nil, err
	}

	if ReadOnly(ctx) {
		return nil, fmt.Errorf("%w: Google Ads click conversion upload", ErrReadOnly)
	}

	if apply {
		ctx = WithoutRetries(ctx)
	}
	var response json.RawMessage
	err = c.doJSON(ctx, http.MethodPost, c.endpoint("customers/"+id+":uploadClickConversions"), request, &response)

	return response, err
}
