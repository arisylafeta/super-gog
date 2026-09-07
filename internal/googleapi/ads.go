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

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsCustomerClientCreate struct {
	DescriptiveName string `json:"descriptiveName"`
	CurrencyCode    string `json:"currencyCode"`
	TimeZone        string `json:"timeZone"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsCreateCustomerClientRequest struct {
	CustomerClient AdsCustomerClientCreate `json:"customerClient"`
	ValidateOnly   bool                    `json:"validateOnly,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsCreateCustomerClientResponse struct {
	ResourceName string `json:"resourceName,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsConversionActionCreate struct {
	Name           string `json:"name"`
	Category       string `json:"category"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	CountingType   string `json:"countingType"`
	PrimaryForGoal bool   `json:"primaryForGoal"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsConversionActionUpdate struct {
	ResourceName   string `json:"resourceName"`
	PrimaryForGoal bool   `json:"primaryForGoal"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsConversionActionOperation struct {
	Create     *AdsConversionActionCreate `json:"create,omitempty"`
	Update     *AdsConversionActionUpdate `json:"update,omitempty"`
	UpdateMask string                     `json:"updateMask,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsMutateConversionActionsRequest struct {
	Operations   []AdsConversionActionOperation `json:"operations"`
	ValidateOnly bool                           `json:"validateOnly,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsMutateConversionActionResult struct {
	ResourceName string `json:"resourceName,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsMutateConversionActionsResponse struct {
	Results []AdsMutateConversionActionResult `json:"results,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsCampaignStatusUpdate struct {
	ResourceName string `json:"resourceName"`
	Status       string `json:"status"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsCampaignOperation struct {
	Update     AdsCampaignStatusUpdate `json:"update"`
	UpdateMask string                  `json:"updateMask"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsMutateCampaignsRequest struct {
	Operations   []AdsCampaignOperation `json:"operations"`
	ValidateOnly bool                   `json:"validateOnly,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsMutateCampaignResult struct {
	ResourceName string `json:"resourceName,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsMutateCampaignsResponse struct {
	Results []AdsMutateCampaignResult `json:"results,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsCustomerConversionGoalUpdate struct {
	ResourceName string `json:"resourceName"`
	Biddable     bool   `json:"biddable"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsCustomerConversionGoalOperation struct {
	Update     AdsCustomerConversionGoalUpdate `json:"update"`
	UpdateMask string                          `json:"updateMask"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsMutateCustomerConversionGoalsRequest struct {
	Operations   []AdsCustomerConversionGoalOperation `json:"operations"`
	ValidateOnly bool                                 `json:"validateOnly,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsMutateCustomerConversionGoalResult struct {
	ResourceName string `json:"resourceName,omitempty"`
}

//nolint:tagliatelle // Google Ads REST resources use lowerCamelCase JSON fields.
type AdsMutateCustomerConversionGoalsResponse struct {
	Results []AdsMutateCustomerConversionGoalResult `json:"results,omitempty"`
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

func (c *AdsClient) CreateCustomerClient(
	ctx context.Context,
	managerCustomerID string,
	request AdsCreateCustomerClientRequest,
) (*AdsCreateCustomerClientResponse, error) {
	id, err := NormalizeAdsCustomerID(managerCustomerID)
	if err != nil {
		return nil, err
	}
	request.CustomerClient.DescriptiveName = strings.TrimSpace(request.CustomerClient.DescriptiveName)
	request.CustomerClient.CurrencyCode = strings.ToUpper(strings.TrimSpace(request.CustomerClient.CurrencyCode))
	request.CustomerClient.TimeZone = strings.TrimSpace(request.CustomerClient.TimeZone)
	if request.CustomerClient.DescriptiveName == "" {
		return nil, errors.New("Google Ads customer descriptive name is required")
	}
	if len(request.CustomerClient.CurrencyCode) != 3 {
		return nil, errors.New("Google Ads customer currency code must be a three-letter code")
	}
	if request.CustomerClient.TimeZone == "" {
		return nil, errors.New("Google Ads customer time zone is required")
	}

	var response AdsCreateCustomerClientResponse
	endpoint := c.endpoint("customers/" + id + ":createCustomerClient")
	if err := c.doJSON(ctx, http.MethodPost, endpoint, request, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *AdsClient) CreateConversionAction(
	ctx context.Context,
	customerID string,
	action AdsConversionActionCreate,
	validateOnly bool,
) (*AdsMutateConversionActionsResponse, error) {
	id, err := NormalizeAdsCustomerID(customerID)
	if err != nil {
		return nil, err
	}
	action.Name = strings.TrimSpace(action.Name)
	action.Category = strings.ToUpper(strings.TrimSpace(action.Category))
	action.Type = strings.ToUpper(strings.TrimSpace(action.Type))
	action.Status = strings.ToUpper(strings.TrimSpace(action.Status))
	action.CountingType = strings.ToUpper(strings.TrimSpace(action.CountingType))
	if action.Name == "" {
		return nil, errors.New("Google Ads conversion action name is required")
	}
	if action.Category == "" {
		return nil, errors.New("Google Ads conversion action category is required")
	}
	if action.Type != "WEBPAGE" {
		return nil, errors.New("Google Ads conversion action type must be WEBPAGE")
	}
	if action.Status != "ENABLED" {
		return nil, errors.New("Google Ads conversion action status must be ENABLED")
	}
	if action.CountingType != "ONE_PER_CLICK" && action.CountingType != "MANY_PER_CLICK" {
		return nil, errors.New("Google Ads conversion action counting type must be ONE_PER_CLICK or MANY_PER_CLICK")
	}

	request := AdsMutateConversionActionsRequest{
		Operations:   []AdsConversionActionOperation{{Create: &action}},
		ValidateOnly: validateOnly,
	}
	var response AdsMutateConversionActionsResponse
	endpoint := c.endpoint("customers/" + id + "/conversionActions:mutate")
	if err := c.doJSON(ctx, http.MethodPost, endpoint, request, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *AdsClient) MakeConversionActionSecondary(
	ctx context.Context,
	customerID string,
	conversionActionID string,
	validateOnly bool,
) (*AdsMutateConversionActionsResponse, error) {
	id, err := NormalizeAdsCustomerID(customerID)
	if err != nil {
		return nil, err
	}
	actionID, err := NormalizeAdsCustomerID(conversionActionID)
	if err != nil {
		return nil, fmt.Errorf("conversion action ID: %w", err)
	}

	resourceName := "customers/" + id + "/conversionActions/" + actionID
	request := AdsMutateConversionActionsRequest{
		Operations: []AdsConversionActionOperation{{
			Update: &AdsConversionActionUpdate{
				ResourceName:   resourceName,
				PrimaryForGoal: false,
			},
			UpdateMask: "primary_for_goal",
		}},
		ValidateOnly: validateOnly,
	}
	var response AdsMutateConversionActionsResponse
	endpoint := c.endpoint("customers/" + id + "/conversionActions:mutate")
	if err := c.doJSON(ctx, http.MethodPost, endpoint, request, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *AdsClient) PauseCampaign(
	ctx context.Context,
	customerID string,
	campaignID string,
	validateOnly bool,
) (*AdsMutateCampaignsResponse, error) {
	id, err := NormalizeAdsCustomerID(customerID)
	if err != nil {
		return nil, err
	}
	campaign, err := NormalizeAdsCustomerID(campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign ID: %w", err)
	}

	resourceName := "customers/" + id + "/campaigns/" + campaign
	request := AdsMutateCampaignsRequest{
		Operations: []AdsCampaignOperation{{
			Update: AdsCampaignStatusUpdate{
				ResourceName: resourceName,
				Status:       "PAUSED",
			},
			UpdateMask: "status",
		}},
		ValidateOnly: validateOnly,
	}
	var response AdsMutateCampaignsResponse
	endpoint := c.endpoint("customers/" + id + "/campaigns:mutate")
	if err := c.doJSON(ctx, http.MethodPost, endpoint, request, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *AdsClient) DisableCustomerConversionGoal(
	ctx context.Context,
	customerID string,
	category string,
	origin string,
	validateOnly bool,
) (*AdsMutateCustomerConversionGoalsResponse, error) {
	id, err := NormalizeAdsCustomerID(customerID)
	if err != nil {
		return nil, err
	}
	category = strings.ToUpper(strings.TrimSpace(category))
	origin = strings.ToUpper(strings.TrimSpace(origin))
	if category == "" {
		return nil, errors.New("Google Ads conversion goal category is required")
	}
	if origin == "" {
		return nil, errors.New("Google Ads conversion goal origin is required")
	}

	resourceName := "customers/" + id + "/customerConversionGoals/" + category + "~" + origin
	request := AdsMutateCustomerConversionGoalsRequest{
		Operations: []AdsCustomerConversionGoalOperation{{
			Update: AdsCustomerConversionGoalUpdate{
				ResourceName: resourceName,
				Biddable:     false,
			},
			UpdateMask: "biddable",
		}},
		ValidateOnly: validateOnly,
	}
	var response AdsMutateCustomerConversionGoalsResponse
	endpoint := c.endpoint("customers/" + id + "/customerConversionGoals:mutate")
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
			Details []struct {
				Errors []struct {
					ErrorCode map[string]string `json:"errorCode"`
					Message   string            `json:"message"`
					Location  struct {
						FieldPathElements []struct {
							FieldName string `json:"fieldName"`
						} `json:"fieldPathElements"`
					} `json:"location"`
				} `json:"errors"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		message := strings.TrimSpace(parsed.Error.Message)
		if detail := adsAPIFailureDetail(parsed.Error.Details); detail != "" {
			message += ": " + detail
		}
		status := strings.TrimSpace(parsed.Error.Status)
		if status != "" {
			return &HTTPStatusError{
				Code:   statusCode,
				Status: status,
				Err:    fmt.Errorf("%w (%d %s): %s", ErrAdsAPI, statusCode, status, message),
			}
		}

		return &HTTPStatusError{
			Code: statusCode,
			Err:  fmt.Errorf("%w (%d): %s", ErrAdsAPI, statusCode, message),
		}
	}

	return &HTTPStatusError{
		Code: statusCode,
		Err:  fmt.Errorf("%w (%d): %s", ErrAdsAPI, statusCode, strings.TrimSpace(string(body))),
	}
}

func adsAPIFailureDetail(details []struct {
	Errors []struct {
		ErrorCode map[string]string `json:"errorCode"`
		Message   string            `json:"message"`
		Location  struct {
			FieldPathElements []struct {
				FieldName string `json:"fieldName"`
			} `json:"fieldPathElements"`
		} `json:"location"`
	} `json:"errors"`
},
) string {
	for _, detail := range details {
		for _, apiError := range detail.Errors {
			code := ""
			for _, value := range apiError.ErrorCode {
				code = strings.TrimSpace(value)
				if code != "" {
					break
				}
			}
			message := strings.TrimSpace(apiError.Message)
			path := make([]string, 0, len(apiError.Location.FieldPathElements))
			for _, element := range apiError.Location.FieldPathElements {
				if field := strings.TrimSpace(element.FieldName); field != "" {
					path = append(path, field)
				}
			}
			parts := make([]string, 0, 3)
			if code != "" {
				parts = append(parts, code)
			}
			if message != "" {
				parts = append(parts, message)
			}
			if len(path) > 0 {
				parts = append(parts, "field "+strings.Join(path, "."))
			}
			if len(parts) > 0 {
				return strings.Join(parts, ": ")
			}
		}
	}

	return ""
}
