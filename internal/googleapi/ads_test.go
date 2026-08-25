package googleapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdsClientListAccessibleCustomers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v25/customers:listAccessibleCustomers" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("developer-token"); got != "developer-token" {
			t.Fatalf("developer-token = %q", got)
		}
		if got := request.Header.Get("login-customer-id"); got != "1234567890" {
			t.Fatalf("login-customer-id = %q", got)
		}
		_, _ = response.Write([]byte(`{"resourceNames":["customers/1234567890"]}`))
	}))
	t.Cleanup(server.Close)

	client := newAdsTestClient(t, server, AdsConfig{
		DeveloperToken:  "developer-token",
		LoginCustomerID: "123-456-7890",
	})
	result, err := client.ListAccessibleCustomers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ResourceNames) != 1 || result.ResourceNames[0] != "customers/1234567890" {
		t.Fatalf("resource names = %#v", result.ResourceNames)
	}
}

func TestAdsClientGetField(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v25/googleAdsFields/metrics.clicks" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"name":"metrics.clicks","category":"METRIC","selectable":true,"dataType":"INT64"}`))
	}))
	t.Cleanup(server.Close)

	client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "developer-token"})
	field, err := client.GetField(context.Background(), "metrics.clicks")
	if err != nil {
		t.Fatal(err)
	}
	if field.Name != "metrics.clicks" || field.Category != "METRIC" || !field.Selectable || field.DataType != "INT64" {
		t.Fatalf("field = %#v", field)
	}
}

func TestAdsClientSearch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/1234567890/googleAds:search" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type = %q", got)
		}
		var body AdsSearchRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Query != "SELECT campaign.id FROM campaign" || body.PageToken != "next" {
			t.Fatalf("body = %#v", body)
		}
		_, _ = response.Write([]byte(`{"results":[{"campaign":{"id":"42"}}],"nextPageToken":"more","fieldMask":"campaign.id"}`))
	}))
	t.Cleanup(server.Close)

	client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "developer-token"})
	result, err := client.Search(context.Background(), "123-456-7890", AdsSearchRequest{
		Query:     " SELECT campaign.id FROM campaign ",
		PageToken: " next ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.NextPageToken != "more" || result.FieldMask != "campaign.id" {
		t.Fatalf("result = %#v", result)
	}
}

func TestAdsClientCreateCustomerClient(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/1234567890:createCustomerClient" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("login-customer-id"); got != "1234567890" {
			t.Fatalf("login-customer-id = %q", got)
		}
		var body AdsCreateCustomerClientRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.ValidateOnly || body.CustomerClient.DescriptiveName != "ReBattery UK Ads" ||
			body.CustomerClient.CurrencyCode != "GBP" || body.CustomerClient.TimeZone != "Europe/London" {
			t.Fatalf("body = %#v", body)
		}
		_, _ = response.Write([]byte(`{"resourceName":"customers/2468013579"}`))
	}))
	t.Cleanup(server.Close)

	client := newAdsTestClient(t, server, AdsConfig{
		DeveloperToken:  "developer-token",
		LoginCustomerID: "1234567890",
	})
	result, err := client.CreateCustomerClient(context.Background(), "123-456-7890", AdsCreateCustomerClientRequest{
		CustomerClient: AdsCustomerClientCreate{
			DescriptiveName: " ReBattery UK Ads ",
			CurrencyCode:    " gbp ",
			TimeZone:        " Europe/London ",
		},
		ValidateOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ResourceName != "customers/2468013579" {
		t.Fatalf("resource name = %q", result.ResourceName)
	}
}

func TestAdsClientCreateConversionAction(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/2468013579/conversionActions:mutate" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body AdsMutateConversionActionsRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.ValidateOnly || len(body.Operations) != 1 {
			t.Fatalf("body = %#v", body)
		}
		action := body.Operations[0].Create
		if action == nil {
			t.Fatal("missing create operation")
		}
		if action.Name != "Quote completed" || action.Category != "REQUEST_QUOTE" || action.Type != "WEBPAGE" ||
			action.Status != "ENABLED" || action.CountingType != "ONE_PER_CLICK" || !action.PrimaryForGoal {
			t.Fatalf("action = %#v", action)
		}
		_, _ = response.Write([]byte(`{"results":[{"resourceName":"customers/2468013579/conversionActions/42"}]}`))
	}))
	t.Cleanup(server.Close)

	client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "developer-token"})
	result, err := client.CreateConversionAction(context.Background(), "246-801-3579", AdsConversionActionCreate{
		Name:           " Quote completed ",
		Category:       " request_quote ",
		Type:           " webpage ",
		Status:         " enabled ",
		CountingType:   " one_per_click ",
		PrimaryForGoal: true,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].ResourceName != "customers/2468013579/conversionActions/42" {
		t.Fatalf("results = %#v", result.Results)
	}
}

func TestAdsClientMakeConversionActionSecondary(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/2468013579/conversionActions:mutate" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body AdsMutateConversionActionsRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.ValidateOnly || len(body.Operations) != 1 {
			t.Fatalf("body = %#v", body)
		}
		operation := body.Operations[0]
		if operation.Update == nil || operation.Update.ResourceName != "customers/2468013579/conversionActions/42" ||
			operation.Update.PrimaryForGoal || operation.UpdateMask != "primary_for_goal" || operation.Create != nil {
			t.Fatalf("operation = %#v", operation)
		}
		_, _ = response.Write([]byte(`{"results":[{"resourceName":"customers/2468013579/conversionActions/42"}]}`))
	}))
	t.Cleanup(server.Close)

	client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "developer-token"})
	result, err := client.MakeConversionActionSecondary(context.Background(), "246-801-3579", "42", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].ResourceName != "customers/2468013579/conversionActions/42" {
		t.Fatalf("results = %#v", result.Results)
	}
}

func TestAdsClientPauseCampaign(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/2468013579/campaigns:mutate" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body AdsMutateCampaignsRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.ValidateOnly || len(body.Operations) != 1 {
			t.Fatalf("body = %#v", body)
		}
		operation := body.Operations[0]
		if operation.Update.ResourceName != "customers/2468013579/campaigns/42" ||
			operation.Update.Status != "PAUSED" || operation.UpdateMask != "status" {
			t.Fatalf("operation = %#v", operation)
		}
		_, _ = response.Write([]byte(`{"results":[{"resourceName":"customers/2468013579/campaigns/42"}]}`))
	}))
	t.Cleanup(server.Close)

	client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "developer-token"})
	result, err := client.PauseCampaign(context.Background(), "246-801-3579", "42", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].ResourceName != "customers/2468013579/campaigns/42" {
		t.Fatalf("results = %#v", result.Results)
	}
}

func TestAdsClientDisableCustomerConversionGoal(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/2468013579/customerConversionGoals:mutate" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body AdsMutateCustomerConversionGoalsRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.ValidateOnly || len(body.Operations) != 1 {
			t.Fatalf("body = %#v", body)
		}
		operation := body.Operations[0]
		if operation.Update.ResourceName != "customers/2468013579/customerConversionGoals/SUBMIT_LEAD_FORM~WEBSITE" ||
			operation.Update.Biddable || operation.UpdateMask != "biddable" {
			t.Fatalf("operation = %#v", operation)
		}
		_, _ = response.Write([]byte(`{"results":[{"resourceName":"customers/2468013579/customerConversionGoals/SUBMIT_LEAD_FORM~WEBSITE"}]}`))
	}))
	t.Cleanup(server.Close)

	client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "developer-token"})
	result, err := client.DisableCustomerConversionGoal(context.Background(), "246-801-3579", " submit_lead_form ", " website ", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].ResourceName != "customers/2468013579/customerConversionGoals/SUBMIT_LEAD_FORM~WEBSITE" {
		t.Fatalf("results = %#v", result.Results)
	}
}

func TestAdsClientAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadRequest)
		_, _ = response.Write([]byte(`{"error":{"code":400,"message":"bad GAQL","status":"INVALID_ARGUMENT","details":[{"errors":[{"errorCode":{"queryError":"BAD_FIELD_NAME"},"message":"Unknown field.","location":{"fieldPathElements":[{"fieldName":"query"}]}}]}]}}`))
	}))
	t.Cleanup(server.Close)

	client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "developer-token"})
	_, err := client.Search(context.Background(), "1234567890", AdsSearchRequest{Query: "bad"})
	if !errors.Is(err, ErrAdsAPI) || !strings.Contains(err.Error(), "INVALID_ARGUMENT") ||
		!strings.Contains(err.Error(), "BAD_FIELD_NAME: Unknown field.: field query") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewAdsClientValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		config AdsConfig
	}{
		{name: "missing developer token", config: AdsConfig{}},
		{name: "invalid API version", config: AdsConfig{DeveloperToken: "token", APIVersion: "latest"}},
		{name: "invalid login customer", config: AdsConfig{DeveloperToken: "token", LoginCustomerID: "abc"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewAdsClient(nil, test.config); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestNormalizeAdsCustomerID(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"1234567890":   "1234567890",
		"123-456-7890": "1234567890",
		"123 456 7890": "1234567890",
	} {
		got, err := NormalizeAdsCustomerID(input)
		if err != nil {
			t.Fatalf("NormalizeAdsCustomerID(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeAdsCustomerID(%q) = %q, want %q", input, got, want)
		}
	}
}

func newAdsTestClient(t *testing.T, server *httptest.Server, config AdsConfig) *AdsClient {
	t.Helper()
	client, err := NewAdsClient(server.Client(), config, WithAdsBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	return client
}
