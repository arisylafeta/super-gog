package cmd

import (
	"context"
	"encoding/json"
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
