package googleapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const defaultDataManagerURL = "https://datamanager.googleapis.com/v1/events:ingest"

var errAdsClickUpload = errors.New("invalid Google Ads click conversion upload")

// Fields an offline click conversion event may carry. Anything else (user data,
// device info, cart data) is refused, so an upload only ever sends a click id, the
// time, value and the transaction id Google uses for deduplication.
var allowedClickEventFields = map[string]bool{
	"adIdentifiers": true, "eventTimestamp": true, "transactionId": true,
	"conversionValue": true, "currency": true, "eventSource": true,
}

var allowedClickIdentifiers = map[string]bool{"gclid": true, "gbraid": true, "wbraid": true}

var allowedConsentStatus = map[string]bool{"CONSENT_GRANTED": true, "CONSENT_DENIED": true}

// ClickConversionDestination is the Google Ads conversion action receiving the events.
type ClickConversionDestination struct {
	CustomerID         string
	LoginCustomerID    string
	ConversionActionID string
}

// PrepareClickConversionUpload turns {"events":[...], "consent":{...}} into a Data
// Manager API events:ingest request for one Google Ads conversion action. Google
// limits Google Ads API click uploads to existing integrations; new ones use this API.
func PrepareClickConversionUpload(destination ClickConversionDestination, body []byte, apply bool) (map[string]any, error) {
	customerID, err := NormalizeAdsCustomerID(destination.CustomerID)
	if err != nil {
		return nil, err
	}

	actionID, err := NormalizeAdsCustomerID(destination.ConversionActionID)
	if err != nil {
		return nil, fmt.Errorf("conversion action ID: %w", err)
	}

	var request map[string]json.RawMessage
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("decode upload JSON: %w", err)
	}

	if request == nil {
		return nil, fmt.Errorf("%w: must be a JSON object", errAdsClickUpload)
	}

	for key := range request {
		if key != "events" && key != "consent" {
			return nil, fmt.Errorf("%w: unsupported field %q", errAdsClickUpload, key)
		}
	}

	var events []map[string]json.RawMessage
	if err := json.Unmarshal(request["events"], &events); err != nil || len(events) == 0 {
		return nil, fmt.Errorf("%w: events must be a nonempty array", errAdsClickUpload)
	}

	if len(events) > 2000 {
		return nil, fmt.Errorf("%w: exceeds 2000 events", errAdsClickUpload)
	}

	for index, event := range events {
		if err := checkClickEvent(event); err != nil {
			return nil, fmt.Errorf("%w: event %d %w", errAdsClickUpload, index, err)
		}
	}

	prepared := map[string]any{
		"destinations": []map[string]any{clickDestination(customerID, destination.LoginCustomerID, actionID)},
		"events":       request["events"],
		"validateOnly": !apply,
	}

	if consent, ok := request["consent"]; ok {
		var fields map[string]string
		if err := json.Unmarshal(consent, &fields); err != nil {
			return nil, fmt.Errorf("%w: consent must be an object", errAdsClickUpload)
		}

		for key, value := range fields {
			if (key != "adUserData" && key != "adPersonalization") || !allowedConsentStatus[value] {
				return nil, fmt.Errorf("%w: unsupported consent %q", errAdsClickUpload, key)
			}
		}

		prepared["consent"] = fields
	}

	return prepared, nil
}

var (
	errClickEventField       = errors.New("has an unsupported field")
	errClickEventIdentifier  = errors.New("needs adIdentifiers with exactly one of gclid, gbraid or wbraid")
	errClickEventTimestamp   = errors.New("needs eventTimestamp")
	errClickEventTransaction = errors.New("needs transactionId for deduplication")
)

func checkClickEvent(event map[string]json.RawMessage) error {
	for key := range event {
		if !allowedClickEventFields[key] {
			return fmt.Errorf("%w %q", errClickEventField, key)
		}
	}

	var identifiers map[string]string
	if err := json.Unmarshal(event["adIdentifiers"], &identifiers); err != nil || len(identifiers) != 1 {
		return errClickEventIdentifier
	}

	for key, value := range identifiers {
		if !allowedClickIdentifiers[key] || value == "" {
			return errClickEventIdentifier
		}
	}

	var timestamp, transaction string
	if json.Unmarshal(event["eventTimestamp"], &timestamp) != nil || timestamp == "" {
		return errClickEventTimestamp
	}

	if json.Unmarshal(event["transactionId"], &transaction) != nil || transaction == "" {
		return errClickEventTransaction
	}

	return nil
}

func clickDestination(customerID, loginCustomerID, actionID string) map[string]any {
	destination := map[string]any{
		"operatingAccount":     map[string]string{"accountType": "GOOGLE_ADS", "accountId": customerID},
		"productDestinationId": actionID,
	}

	if login, err := NormalizeAdsCustomerID(loginCustomerID); err == nil && login != customerID {
		destination["loginAccount"] = map[string]string{"accountType": "GOOGLE_ADS", "accountId": login}
	}

	return destination
}

// UploadClickConversions sends offline click conversions through the Data Manager API.
// Applied uploads are not retried automatically; Google drops a repeated transactionId
// for the same conversion action. A request is all-or-nothing (fast fail).
func (c *AdsClient) UploadClickConversions(ctx context.Context, destination ClickConversionDestination, body []byte, apply bool) (json.RawMessage, error) {
	if destination.LoginCustomerID == "" {
		destination.LoginCustomerID = c.loginCustomerID
	}

	request, err := PrepareClickConversionUpload(destination, body, apply)
	if err != nil {
		return nil, err
	}

	if ReadOnly(ctx) {
		return nil, fmt.Errorf("%w: Google Ads click conversion upload", ErrReadOnly)
	}

	if apply {
		ctx = WithoutRetries(ctx)
	}

	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode Data Manager request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.dataManagerURL, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("build Data Manager request: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("send Data Manager request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxAdsResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Data Manager response: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, adsAPIError(response.StatusCode, responseBody)
	}

	return json.RawMessage(responseBody), nil
}
