package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/steipete/gogcli/internal/app"
	"github.com/steipete/gogcli/internal/googleapi"
)

func TestAdsCustomersJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v25/customers:listAccessibleCustomers" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"resourceNames":["customers/1234567890","customers/9876543210"]}`))
	}))
	t.Cleanup(server.Close)
	client := newAdsCommandTestClient(t, server)

	setAdsCommandTestEnv(t)
	result := executeWithAdsTestClient(t, []string{
		"--json", "--account", "ads@example.com", "ads", "customers", "--login-customer-id", "100-162-3054",
	}, client, func(config googleapi.AdsConfig) {
		if config.DeveloperToken != "cli-developer-token" || config.LoginCustomerID != "100-162-3054" || config.APIVersion != "v25" {
			t.Fatalf("config = %#v", config)
		}
	})
	if result.err != nil {
		t.Fatalf("execute: %v\nstderr: %s", result.err, result.stderr)
	}

	var output struct {
		CustomerIDs   []string `json:"customerIds"`
		CustomerCount int      `json:"customerCount"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &output); err != nil {
		t.Fatalf("decode output: %v\n%s", err, result.stdout)
	}
	if output.CustomerCount != 2 || len(output.CustomerIDs) != 2 || output.CustomerIDs[0] != "1234567890" {
		t.Fatalf("output = %#v", output)
	}
}

func TestAdsFieldPlain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"resourceName":"googleAdsFields/metrics.clicks","name":"metrics.clicks","category":"METRIC","dataType":"INT64","selectable":true,"filterable":true}`))
	}))
	t.Cleanup(server.Close)
	client := newAdsCommandTestClient(t, server)

	setAdsCommandTestEnv(t)
	result := executeWithAdsTestClient(t, []string{
		"--plain", "--account", "ads@example.com", "ads", "fields", "metrics.clicks",
	}, client, nil)
	if result.err != nil {
		t.Fatalf("execute: %v\nstderr: %s", result.err, result.stderr)
	}
	for _, want := range []string{"name\tmetrics.clicks", "category\tMETRIC", "selectable\ttrue"} {
		if !strings.Contains(result.stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, result.stdout)
		}
	}
}

func TestAdsQueryJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v25/customers/1234567890/googleAds:search" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"results":[{"campaign":{"id":"42","name":"Pilot"}}],"fieldMask":"campaign.id,campaign.name"}`))
	}))
	t.Cleanup(server.Close)
	client := newAdsCommandTestClient(t, server)

	setAdsCommandTestEnv(t)
	result := executeWithAdsTestClient(t, []string{
		"--json", "--account", "ads@example.com", "ads", "query", "123-456-7890",
		"--gaql", "SELECT campaign.id, campaign.name FROM campaign",
	}, client, nil)
	if result.err != nil {
		t.Fatalf("execute: %v\nstderr: %s", result.err, result.stderr)
	}

	var output struct {
		CustomerID  string            `json:"customerId"`
		ResultCount int               `json:"resultCount"`
		Results     []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &output); err != nil {
		t.Fatalf("decode output: %v\n%s", err, result.stdout)
	}
	if output.CustomerID != "1234567890" || output.ResultCount != 1 || len(output.Results) != 1 {
		t.Fatalf("output = %#v", output)
	}
}

func TestAdsAccountCreateValidateOnlyJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/1001623054:createCustomerClient" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body googleapi.AdsCreateCustomerClientRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.ValidateOnly || body.CustomerClient.CurrencyCode != "GBP" || body.CustomerClient.TimeZone != "Europe/London" {
			t.Fatalf("body = %#v", body)
		}
		_, _ = response.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	client := newAdsCommandTestClient(t, server)

	setAdsCommandTestEnv(t)
	result := executeWithAdsTestClient(t, []string{
		"--json", "--account", "ads@example.com", "ads", "account", "create", "100-162-3054",
		"--name", "ReBattery UK Ads", "--currency-code", "gbp", "--time-zone", "Europe/London", "--validate-only",
	}, client, func(config googleapi.AdsConfig) {
		if config.LoginCustomerID != "1001623054" {
			t.Fatalf("login customer = %q", config.LoginCustomerID)
		}
	})
	if result.err != nil {
		t.Fatalf("execute: %v\nstderr: %s", result.err, result.stderr)
	}
	if !strings.Contains(result.stdout, `"validatedOnly": true`) {
		t.Fatalf("stdout = %s", result.stdout)
	}
}

func TestAdsConversionCreateValidateOnlyJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/2468013579/conversionActions:mutate" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body googleapi.AdsMutateConversionActionsRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.ValidateOnly || len(body.Operations) != 1 || body.Operations[0].Create == nil || body.Operations[0].Create.Category != "CONTACT" {
			t.Fatalf("body = %#v", body)
		}
		_, _ = response.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	client := newAdsCommandTestClient(t, server)

	setAdsCommandTestEnv(t)
	result := executeWithAdsTestClient(t, []string{
		"--json", "--account", "ads@example.com", "ads", "conversion", "create", "246-801-3579",
		"--login-customer-id", "100-162-3054", "--name", "Marketplace action", "--category", "CONTACT", "--validate-only",
	}, client, nil)
	if result.err != nil {
		t.Fatalf("execute: %v\nstderr: %s", result.err, result.stderr)
	}
	if !strings.Contains(result.stdout, `"validatedOnly": true`) {
		t.Fatalf("stdout = %s", result.stdout)
	}
}

func TestAdsConversionSecondaryValidateOnlyJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/2468013579/conversionActions:mutate" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body googleapi.AdsMutateConversionActionsRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.ValidateOnly || len(body.Operations) != 1 || body.Operations[0].Update == nil || body.Operations[0].Update.PrimaryForGoal {
			t.Fatalf("body = %#v", body)
		}
		_, _ = response.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	client := newAdsCommandTestClient(t, server)

	setAdsCommandTestEnv(t)
	result := executeWithAdsTestClient(t, []string{
		"--json", "--account", "ads@example.com", "ads", "conversion", "secondary", "246-801-3579", "42",
		"--login-customer-id", "100-162-3054", "--validate-only",
	}, client, nil)
	if result.err != nil {
		t.Fatalf("execute: %v\nstderr: %s", result.err, result.stderr)
	}
	if !strings.Contains(result.stdout, `"primaryForGoal": false`) || !strings.Contains(result.stdout, `"validatedOnly": true`) {
		t.Fatalf("stdout = %s", result.stdout)
	}
}

func TestAdsCampaignPauseValidateOnlyJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/2468013579/campaigns:mutate" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body googleapi.AdsMutateCampaignsRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.ValidateOnly || len(body.Operations) != 1 || body.Operations[0].Update.Status != "PAUSED" {
			t.Fatalf("body = %#v", body)
		}
		_, _ = response.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	client := newAdsCommandTestClient(t, server)

	setAdsCommandTestEnv(t)
	result := executeWithAdsTestClient(t, []string{
		"--json", "--account", "ads@example.com", "ads", "campaign", "pause", "246-801-3579", "42",
		"--login-customer-id", "100-162-3054", "--validate-only",
	}, client, nil)
	if result.err != nil {
		t.Fatalf("execute: %v\nstderr: %s", result.err, result.stderr)
	}
	if !strings.Contains(result.stdout, `"validatedOnly": true`) || !strings.Contains(result.stdout, `"status": "PAUSED"`) {
		t.Fatalf("stdout = %s", result.stdout)
	}
}

func TestAdsGoalDisableValidateOnlyJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v25/customers/2468013579/customerConversionGoals:mutate" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body googleapi.AdsMutateCustomerConversionGoalsRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.ValidateOnly || len(body.Operations) != 1 || body.Operations[0].Update.Biddable {
			t.Fatalf("body = %#v", body)
		}
		_, _ = response.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	client := newAdsCommandTestClient(t, server)

	setAdsCommandTestEnv(t)
	result := executeWithAdsTestClient(t, []string{
		"--json", "--account", "ads@example.com", "ads", "goal", "disable", "246-801-3579",
		"--login-customer-id", "100-162-3054", "--category", "SUBMIT_LEAD_FORM", "--origin", "WEBSITE", "--validate-only",
	}, client, nil)
	if result.err != nil {
		t.Fatalf("execute: %v\nstderr: %s", result.err, result.stderr)
	}
	if !strings.Contains(result.stdout, `"biddable": false`) || !strings.Contains(result.stdout, `"validatedOnly": true`) {
		t.Fatalf("stdout = %s", result.stdout)
	}
}

func TestAdsWritesRequireForceAndRespectReadOnly(t *testing.T) {
	setAdsCommandTestEnv(t)
	called := false
	runtime := &app.Runtime{Services: app.Services{
		Ads: func(context.Context, string, googleapi.AdsConfig) (*googleapi.AdsClient, error) {
			called = true
			return nil, nil
		},
	}}

	result := executeWithTestRuntime(t, []string{
		"--no-input", "--account", "ads@example.com", "ads", "account", "create", "1001623054",
		"--name", "ReBattery UK Ads", "--currency-code", "GBP", "--time-zone", "Europe/London",
	}, runtime)
	if result.err == nil || !strings.Contains(result.err.Error(), "without --force") {
		t.Fatalf("force error = %v", result.err)
	}
	if called {
		t.Fatal("Ads service called before force confirmation")
	}

	result = executeWithTestRuntime(t, []string{
		"--readonly", "--force", "--account", "ads@example.com", "ads", "conversion", "create", "2468013579",
		"--name", "Quote completed", "--category", "REQUEST_QUOTE",
	}, runtime)
	if result.err == nil || !errors.Is(result.err, googleapi.ErrReadOnly) {
		t.Fatalf("read-only error = %v", result.err)
	}
	if called {
		t.Fatal("Ads service called under --readonly")
	}

	result = executeWithTestRuntime(t, []string{
		"--readonly", "--force", "--account", "ads@example.com", "ads", "campaign", "pause", "2468013579", "42",
	}, runtime)
	if result.err == nil || !errors.Is(result.err, googleapi.ErrReadOnly) {
		t.Fatalf("read-only campaign error = %v", result.err)
	}
	if called {
		t.Fatal("Ads service called under read-only campaign pause")
	}
}

func TestAdsRequiresDeveloperTokenBeforeService(t *testing.T) {
	t.Setenv("GOG_ADS_DEVELOPER_TOKEN", "")
	t.Setenv("GOOGLE_ADS_DEVELOPER_TOKEN", "")
	called := false
	result := executeWithTestRuntime(t, []string{
		"--account", "ads@example.com", "ads", "customers",
	}, &app.Runtime{Services: app.Services{
		Ads: func(context.Context, string, googleapi.AdsConfig) (*googleapi.AdsClient, error) {
			called = true
			return nil, nil
		},
	}})
	if result.err == nil || !strings.Contains(result.err.Error(), "GOG_ADS_DEVELOPER_TOKEN") {
		t.Fatalf("error = %v", result.err)
	}
	if called {
		t.Fatal("Ads service called without developer token")
	}
}

func executeWithAdsTestClient(
	t *testing.T,
	args []string,
	client *googleapi.AdsClient,
	inspect func(googleapi.AdsConfig),
) executeTestResult {
	t.Helper()
	return executeWithTestRuntime(t, args, &app.Runtime{Services: app.Services{
		Ads: func(_ context.Context, account string, config googleapi.AdsConfig) (*googleapi.AdsClient, error) {
			if account != "ads@example.com" {
				t.Fatalf("account = %q", account)
			}
			if inspect != nil {
				inspect(config)
			}
			return client, nil
		},
	}})
}

func newAdsCommandTestClient(t *testing.T, server *httptest.Server) *googleapi.AdsClient {
	t.Helper()
	client, err := googleapi.NewAdsClient(
		server.Client(),
		googleapi.AdsConfig{DeveloperToken: "server-developer-token"},
		googleapi.WithAdsBaseURL(server.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func setAdsCommandTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GOG_ADS_DEVELOPER_TOKEN", "cli-developer-token")
	t.Setenv("GOOGLE_ADS_DEVELOPER_TOKEN", "")
	t.Setenv("GOG_ADS_LOGIN_CUSTOMER_ID", "")
	t.Setenv("GOOGLE_ADS_LOGIN_CUSTOMER_ID", "")
}
