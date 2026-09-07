package googleapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdsMutationModes(t *testing.T) {
	t.Parallel()

	for _, apply := range []bool{false, true} {
		t.Run(map[bool]string{false: "validate", true: "apply"}[apply], func(t *testing.T) {
			t.Parallel()

			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++

				if r.Method != "POST" || r.URL.Path != "/v25/customers/1234567890/googleAds:mutate" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}

				var body map[string]json.RawMessage

				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}

				if string(body["validateOnly"]) != map[bool]string{false: "true", true: "false"}[apply] || string(body["partialFailure"]) != "false" {
					t.Errorf("unsafe flags: %s", body)
				}

				_, _ = w.Write([]byte(`{"mutateOperationResponses":[]}`))
			}))

			defer server.Close()
			client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "test"})
			_, err := client.Mutate(context.Background(), "123-456-7890", "googleAds", []byte(`{"mutateOperations":[{"campaignOperation":{"create":{"name":"Pilot","status":"PAUSED"}}}],"validateOnly":false,"partialFailure":true}`), apply)
			if err != nil {
				t.Fatal(err)
			}

			if calls != 1 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}

func TestPrepareAdsMutationRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`null`, `[]`, `{}`, `{"operations":[]}`, `{"operations":[null]}`, `{"operations":[{}]}`, `{"operations":[{"create":{}}],"customerId":"999"}`, `{"operations":[{"create":{}}]} {}`} {
		if _, err := PrepareAdsMutation("campaigns", []byte(body), false); err == nil {
			t.Errorf("accepted %s", body)
		}
	}

	if _, err := PrepareAdsMutation("../customers", []byte(`{"operations":[{"create":{}}]}`), false); err == nil {
		t.Error("accepted path injection")
	}
}

type adsMutationTransport func(*http.Request) (*http.Response, error)

func (f adsMutationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAdsMutationWriteTransportGuards(t *testing.T) {
	t.Parallel()

	calls := 0
	transport := adsMutationTransport(func(r *http.Request) (*http.Response, error) {
		calls++

		if !retriesDisabled(r.Context()) {
			t.Error("write allowed automatic retry")
		}

		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	client, err := NewAdsClient(&http.Client{Transport: transport}, AdsConfig{DeveloperToken: "test"})
	if err != nil {
		t.Fatal(err)
	}

	body := []byte(`{"operations":[{"create":{"name":"Pilot"}}]}`)

	if _, err = client.Mutate(WithReadOnly(context.Background(), true), "1234567890", "campaigns", body, true); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("readonly: %v", err)
	}

	if calls != 0 {
		t.Fatal("readonly reached network")
	}

	if _, err = client.Mutate(context.Background(), "1234567890", "campaigns", body, true); err != nil {
		t.Fatal(err)
	}

	if calls != 1 {
		t.Fatalf("calls %d", calls)
	}
}

func TestAdsScheduleExperiment(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v25/customers/1234567890/experiments/42:scheduleExperiment" {
			t.Errorf("path %s", r.URL.Path)
		}

		var body map[string]bool

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}

		if !body["validateOnly"] {
			t.Error("validation omitted")
		}

		_, _ = w.Write([]byte(`{}`))
	}))

	defer server.Close()
	client := newAdsTestClient(t, server, AdsConfig{DeveloperToken: "test"})

	if _, err := client.ScheduleExperiment(context.Background(), "1234567890", "42", false); err != nil {
		t.Fatal(err)
	}

	if _, err := client.ScheduleExperiment(WithReadOnly(context.Background(), true), "1234567890", "42", true); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("readonly: %v", err)
	}
}
