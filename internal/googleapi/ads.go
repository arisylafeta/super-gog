package googleapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/steipete/gogcli/internal/googleauth"
)

const (
	DefaultAdsAPIVersion = "v25"
	defaultAdsBaseURL    = "https://googleads.googleapis.com"
	maxAdsResponseBytes  = 64 << 20
)

var ErrAdsAPI = errors.New("google ads API error")

type AdsConfig struct {
	DeveloperToken  string
	LoginCustomerID string
	APIVersion      string
}

type AdsClient struct {
	baseURL         string
	client          *http.Client
	developerToken  string
	loginCustomerID string
	apiVersion      string
}

type AdsClientOption func(*AdsClient)

func WithAdsBaseURL(baseURL string) AdsClientOption {
	return func(client *AdsClient) {
		if value := strings.TrimSpace(baseURL); value != "" {
			client.baseURL = strings.TrimRight(value, "/")
		}
	}
}

func NewAdsClient(client *http.Client, config AdsConfig, opts ...AdsClientOption) (*AdsClient, error) {
	developerToken := strings.TrimSpace(config.DeveloperToken)
	if developerToken == "" {
		return nil, errors.New("Google Ads developer token is required")
	}

	apiVersion, err := normalizeAdsAPIVersion(config.APIVersion)
	if err != nil {
		return nil, err
	}

	loginCustomerID := ""
	if strings.TrimSpace(config.LoginCustomerID) != "" {
		loginCustomerID, err = NormalizeAdsCustomerID(config.LoginCustomerID)
		if err != nil {
			return nil, fmt.Errorf("login customer ID: %w", err)
		}
	}

	if client == nil {
		client = http.DefaultClient
	}

	adsClient := &AdsClient{
		baseURL:         defaultAdsBaseURL,
		client:          client,
		developerToken:  developerToken,
		loginCustomerID: loginCustomerID,
		apiVersion:      apiVersion,
	}
	for _, opt := range opts {
		opt(adsClient)
	}

	return adsClient, nil
}

func NewAdsClientForAccount(ctx context.Context, email string, config AdsConfig, opts ...AdsClientOption) (*AdsClient, error) {
	client, err := NewHTTPClient(ctx, googleauth.ServiceAds, email)
	if err != nil {
		return nil, err
	}

	return NewAdsClient(client, config, opts...)
}

// NormalizeAdsCustomerID accepts the formatted and unformatted forms used by
// the Google Ads UI and returns the digits-only API representation.
func NormalizeAdsCustomerID(value string) (string, error) {
	normalized := strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(value))
	if normalized == "" {
		return "", errors.New("customer ID is required")
	}
	for _, char := range normalized {
		if char < '0' || char > '9' {
			return "", errors.New("customer ID must contain only digits or hyphens")
		}
	}

	return normalized, nil
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsAccessibleCustomersResponse struct {
	ResourceNames []string `json:"resourceNames,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsField struct {
	ResourceName       string   `json:"resourceName,omitempty"`
	Name               string   `json:"name,omitempty"`
	Category           string   `json:"category,omitempty"`
	Selectable         bool     `json:"selectable,omitempty"`
	Filterable         bool     `json:"filterable,omitempty"`
	Sortable           bool     `json:"sortable,omitempty"`
	SelectableWith     []string `json:"selectableWith,omitempty"`
	AttributeResources []string `json:"attributeResources,omitempty"`
	Metrics            []string `json:"metrics,omitempty"`
	Segments           []string `json:"segments,omitempty"`
	EnumValues         []string `json:"enumValues,omitempty"`
	DataType           string   `json:"dataType,omitempty"`
	TypeURL            string   `json:"typeUrl,omitempty"`
	IsRepeated         bool     `json:"isRepeated,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsSearchRequest struct {
	Query     string `json:"query"`
	PageToken string `json:"pageToken,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsSearchResponse struct {
	Results                  []json.RawMessage `json:"results,omitempty"`
	NextPageToken            string            `json:"nextPageToken,omitempty"`
	TotalResultsCount        string            `json:"totalResultsCount,omitempty"`
	FieldMask                string            `json:"fieldMask,omitempty"`
	SummaryRow               json.RawMessage   `json:"summaryRow,omitempty"`
	QueryResourceConsumption string            `json:"queryResourceConsumption,omitempty"`
}

func (c *AdsClient) ListAccessibleCustomers(ctx context.Context) (*AdsAccessibleCustomersResponse, error) {
	var response AdsAccessibleCustomersResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("customers:listAccessibleCustomers"), nil, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *AdsClient) GetField(ctx context.Context, resourceOrField string) (*AdsField, error) {
	name := strings.TrimSpace(resourceOrField)
	if name == "" {
		return nil, errors.New("Google Ads resource or field is required")
	}
	if strings.Contains(name, "/") {
		return nil, errors.New("Google Ads resource or field must not contain a slash")
	}

	var field AdsField
	endpoint := c.endpoint("googleAdsFields/" + url.PathEscape(name))
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &field); err != nil {
		return nil, err
	}

	return &field, nil
}

func (c *AdsClient) Search(ctx context.Context, customerID string, request AdsSearchRequest) (*AdsSearchResponse, error) {
	id, err := NormalizeAdsCustomerID(customerID)
	if err != nil {
		return nil, err
	}
	request.Query = strings.TrimSpace(request.Query)
	if request.Query == "" {
		return nil, errors.New("Google Ads query is required")
	}
	request.PageToken = strings.TrimSpace(request.PageToken)

	var response AdsSearchResponse
	endpoint := c.endpoint("customers/" + id + "/googleAds:search")
	if err := c.doJSON(ctx, http.MethodPost, endpoint, request, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *AdsClient) endpoint(path string) string {
	return c.baseURL + "/" + c.apiVersion + "/" + strings.TrimLeft(path, "/")
}

func (c *AdsClient) doJSON(ctx context.Context, method, endpoint string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Google Ads API request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("build Google Ads API request: %w", err)
	}
	request.Header.Set("developer-token", c.developerToken)
	if c.loginCustomerID != "" {
		request.Header.Set("login-customer-id", c.loginCustomerID)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("send Google Ads API request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxAdsResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read Google Ads API response: %w", err)
	}
	if len(responseBody) > maxAdsResponseBytes {
		return errors.New("Google Ads API response exceeds 64 MiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return adsAPIError(response.StatusCode, responseBody)
	}
	if err := json.Unmarshal(responseBody, out); err != nil {
		return fmt.Errorf("decode Google Ads API response: %w", err)
	}

	return nil
}

func normalizeAdsAPIVersion(value string) (string, error) {
	version := strings.TrimSpace(value)
	if version == "" {
		version = DefaultAdsAPIVersion
	}
	if len(version) < 2 || version[0] != 'v' {
		return "", errors.New("Google Ads API version must look like v25")
	}
	if _, err := strconv.ParseUint(version[1:], 10, 16); err != nil {
		return "", errors.New("Google Ads API version must look like v25")
	}

	return version, nil
}

func adsAPIError(statusCode int, body []byte) error {
	var parsed struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		status := strings.TrimSpace(parsed.Error.Status)
		if status != "" {
			return &HTTPStatusError{
				Code:   statusCode,
				Status: status,
				Err:    fmt.Errorf("%w (%d %s): %s", ErrAdsAPI, statusCode, status, parsed.Error.Message),
			}
		}

		return &HTTPStatusError{
			Code: statusCode,
			Err:  fmt.Errorf("%w (%d): %s", ErrAdsAPI, statusCode, parsed.Error.Message),
		}
	}

	return &HTTPStatusError{
		Code: statusCode,
		Err:  fmt.Errorf("%w (%d): %s", ErrAdsAPI, statusCode, strings.TrimSpace(string(body))),
	}
}
