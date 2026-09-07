package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steipete/gogcli/internal/app"
	"github.com/steipete/gogcli/internal/googleapi"
)

func TestAdsMutateGuards(t *testing.T) {
	body := filepath.Join(t.TempDir(), "request.json")

	if err := os.WriteFile(body, []byte(`{"operations":[{"create":{"name":"Pilot"}}],"validateOnly":false}`), 0o600); err != nil {
		t.Fatal(err)
	}

	setAdsCommandTestEnv(t)
	called := false
	runtime := &app.Runtime{Services: app.Services{Ads: func(context.Context, string, googleapi.AdsConfig) (*googleapi.AdsClient, error) {
		called = true

		return nil, googleapi.ErrReadOnly
	}}}

	for _, args := range [][]string{
		{"--readonly", "--force", "--account", "ads@example.com", "ads", "mutate", "1234567890", "--service", "campaigns", "--body-file", body},
		{"--no-input", "--account", "ads@example.com", "ads", "mutate", "1234567890", "--service", "campaigns", "--body-file", body, "--apply"},
	} {
		result := executeWithTestRuntime(t, args, runtime)

		if result.err == nil {
			t.Fatal("expected guard error")
		}

		if !errors.Is(result.err, googleapi.ErrReadOnly) && !strings.Contains(result.err.Error(), "without --force") {
			t.Fatalf("unexpected error: %v", result.err)
		}

		if called {
			t.Fatal("auth attempted before guard")
		}

	}

	result := executeWithTestRuntime(t, []string{"--dry-run", "--json", "ads", "mutate", "1234567890", "--service", "campaigns", "--body-file", body}, runtime)

	if called {
		t.Fatal("dry-run contacted service")
	}

	if !strings.Contains(result.stdout, `"validateOnly": true`) {
		t.Fatalf("dry-run output: %s error: %v", result.stdout, result.err)
	}
}

func TestAdsMutateDefaultValidation(t *testing.T) {
	body := filepath.Join(t.TempDir(), "request.json")

	if err := os.WriteFile(body, []byte(`{"operations":[{"create":{"name":"Pilot"}}],"validateOnly":false}`), 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v25/customers/1234567890/campaigns:mutate" {
			t.Errorf("path %s", r.URL.Path)
		}

		var payload map[string]json.RawMessage

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}

		if string(payload["validateOnly"]) != "true" {
			t.Error("file bypassed validation default")
		}

		_, _ = w.Write([]byte(`{}`))
	}))

	defer server.Close()
	setAdsCommandTestEnv(t)
	result := executeWithAdsTestClient(t, []string{"--json", "--no-input", "--account", "ads@example.com", "ads", "mutate", "1234567890", "--service", "campaigns", "--body-file", body}, newAdsCommandTestClient(t, server), nil)

	if result.err != nil {
		t.Fatal(result.err)
	}

	if !strings.Contains(result.stdout, `"validatedOnly": true`) {
		t.Fatalf("output %s", result.stdout)
	}
}
