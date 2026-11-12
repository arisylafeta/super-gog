package googleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const validUpload = `{"conversions":[{"gclid":"Cj0abc","conversionAction":"customers/1234567890/conversionActions/42","conversionDateTime":"2026-11-12 10:00:00+00:00","conversionValue":1200,"currencyCode":"GBP","orderId":"deal-1"}]}`

func TestUploadClickConversionsModes(t *testing.T) {
	t.Parallel()

	for _, apply := range []bool{false, true} {
		t.Run(map[bool]string{false: "validate", true: "apply"}[apply], func(t *testing.T) {
			t.Parallel()
			calls := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++

				if r.Method != "POST" || r.URL.Path != "/v25/customers/1234567890:uploadClickConversions" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}

				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}

				if string(body["validateOnly"]) != map[bool]string{false: "true", true: "false"}[apply] || string(body["partialFailure"]) != "true" {
					t.Errorf("unsafe flags: %s", body)
				}
				_, _ = w.Write([]byte(`{"results":[{}]}`))
			}))
			defer server.Close()

			client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "test"})
			if _, err := client.UploadClickConversions(context.Background(), "123-456-7890", []byte(validUpload), apply); err != nil {
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
		`null`, `{}`, `{"conversions":[]}`,
		`{"conversions":[{"conversionAction":"customers/1234567890/conversionActions/42","conversionDateTime":"x","orderId":"o"}]}`,
		`{"conversions":[{"gclid":"a","gbraid":"b","conversionAction":"customers/1234567890/conversionActions/42","conversionDateTime":"x","orderId":"o"}]}`,
		`{"conversions":[{"gclid":"a","conversionAction":"customers/999/conversionActions/42","conversionDateTime":"x","orderId":"o"}]}`,
		`{"conversions":[{"gclid":"a","conversionAction":"customers/1234567890/conversionActions/42","conversionDateTime":"x"}]}`,
		`{"conversions":[{"gclid":"a","conversionAction":"customers/1234567890/conversionActions/42","conversionDateTime":"x","orderId":"o","userIdentifiers":[]}]}`,
		`{"conversions":[{"gclid":"a","conversionAction":"customers/1234567890/conversionActions/42","conversionDateTime":"x","orderId":"o"}],"customerId":"1"}`,
	} {
		if _, err := PrepareClickConversionUpload("1234567890", []byte(body), false); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
}

func TestUploadClickConversionsReadOnly(t *testing.T) {
	t.Parallel()

	client := newAdsTestClient(t, httptest.NewServer(http.NotFoundHandler()), AdsConfig{DeveloperToken: "test"})
	if _, err := client.UploadClickConversions(WithReadOnly(context.Background(), true), "1234567890", []byte(validUpload), true); err == nil {
		t.Fatal("read-only upload accepted")
	}
}
