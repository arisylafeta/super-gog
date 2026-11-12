package googleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const validUpload = `{"events":[{"adIdentifiers":{"gclid":"Cj0abc"},"eventTimestamp":"2026-11-12T10:00:00Z","transactionId":"deal-1","conversionValue":1200.5,"currency":"GBP","eventSource":"WEB"}],"consent":{"adUserData":"CONSENT_GRANTED","adPersonalization":"CONSENT_GRANTED"}}`

var testDestination = ClickConversionDestination{CustomerID: "830-499-5670", LoginCustomerID: "1001623054", ConversionActionID: "42"}

func TestUploadClickConversionsSendsDataManagerIngest(t *testing.T) {
	t.Parallel()

	for _, apply := range []bool{false, true} {
		t.Run(map[bool]string{false: "validate", true: "apply"}[apply], func(t *testing.T) {
			t.Parallel()

			calls := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++

				if r.Method != http.MethodPost || r.URL.Path != "/v1/events:ingest" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}

				if r.Header.Get("developer-token") != "" {
					t.Error("Data Manager request carries the Ads developer token")
				}

				var body struct {
					Destinations []struct {
						OperatingAccount     map[string]string `json:"operatingAccount"`
						LoginAccount         map[string]string `json:"loginAccount"`
						ProductDestinationID string            `json:"productDestinationId"`
					} `json:"destinations"`
					ValidateOnly bool              `json:"validateOnly"`
					Consent      map[string]string `json:"consent"`
				}

				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}

				if body.ValidateOnly == apply || len(body.Destinations) != 1 {
					t.Errorf("unsafe request: %+v", body)
				}

				destination := body.Destinations[0]
				if destination.OperatingAccount["accountId"] != "8304995670" || destination.LoginAccount["accountId"] != "1001623054" || destination.ProductDestinationID != "42" {
					t.Errorf("destination %+v", destination)
				}

				if body.Consent["adUserData"] != "CONSENT_GRANTED" {
					t.Errorf("consent %+v", body.Consent)
				}

				_, _ = w.Write([]byte(`{"requestId":"r1"}`))
			}))
			defer server.Close()

			client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "test"})
			WithDataManagerURL(server.URL + "/v1/events:ingest")(client)

			if _, err := client.UploadClickConversions(context.Background(), testDestination, []byte(validUpload), apply); err != nil {
				t.Fatal(err)
			}

			if calls != 1 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}

func TestPrepareClickConversionUploadRejects(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`null`, `{}`, `{"events":[]}`,
		`{"events":[{"eventTimestamp":"x","transactionId":"o"}]}`,
		`{"events":[{"adIdentifiers":{"gclid":"a","gbraid":"b"},"eventTimestamp":"x","transactionId":"o"}]}`,
		`{"events":[{"adIdentifiers":{"email":"a"},"eventTimestamp":"x","transactionId":"o"}]}`,
		`{"events":[{"adIdentifiers":{"gclid":"a"},"eventTimestamp":"x"}]}`,
		`{"events":[{"adIdentifiers":{"gclid":"a"},"eventTimestamp":"x","transactionId":"o","userData":{}}]}`,
		`{"events":[{"adIdentifiers":{"gclid":"a"},"eventTimestamp":"x","transactionId":"o"}],"destinations":[]}`,
		`{"events":[{"adIdentifiers":{"gclid":"a"},"eventTimestamp":"x","transactionId":"o"}],"consent":{"adUserData":"YES"}}`,
	} {
		if _, err := PrepareClickConversionUpload(testDestination, []byte(body), false); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
}

func TestUploadClickConversionsReadOnly(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "test"})
	if _, err := client.UploadClickConversions(WithReadOnly(context.Background(), true), testDestination, []byte(validUpload), true); err == nil {
		t.Fatal("read-only upload accepted")
	}
}
