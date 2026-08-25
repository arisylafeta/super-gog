package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/steipete/gogcli/internal/googleapi"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

type AdsCmd struct {
	Customers  AdsCustomersCmd  `cmd:"" name:"customers" aliases:"accounts" help:"List directly accessible Google Ads customers"`
	Fields     AdsFieldsCmd     `cmd:"" name:"fields" aliases:"field" help:"Inspect a Google Ads resource or field"`
	Query      AdsQueryCmd      `cmd:"" name:"query" aliases:"search,report" help:"Run a read-only Google Ads Query Language query"`
	Account    AdsAccountCmd    `cmd:"" name:"account" aliases:"client" help:"Manage Google Ads client accounts"`
	Conversion AdsConversionCmd `cmd:"" name:"conversion" aliases:"conversions" help:"Manage Google Ads website conversion actions"`
}

type AdsConnectionFlags struct {
	LoginCustomerID string `name:"login-customer-id" aliases:"manager-customer-id,mcc" help:"Manager customer ID used to access a client account (digits or hyphenated)"`
	APIVersion      string `name:"api-version" help:"Google Ads API version" default:"v25" env:"GOG_ADS_API_VERSION"`
}

type AdsCustomersCmd struct {
	AdsConnectionFlags `embed:""`
	FailEmpty          bool `name:"fail-empty" aliases:"non-empty,require-results" help:"Exit with code 3 if no customers are accessible"`
}

func (c *AdsCustomersCmd) Run(ctx context.Context, flags *RootFlags) error {
	client, err := requireAdsClient(ctx, flags, c.AdsConnectionFlags)
	if err != nil {
		return err
	}
	response, err := client.ListAccessibleCustomers(ctx)
	if err != nil {
		return err
	}

	resourceNames := response.ResourceNames
	if outfmt.IsJSON(ctx) {
		if err := outfmt.WriteJSON(ctx, stdoutWriter(ctx), map[string]any{
			"resourceNames": resourceNames,
			"customerIds":   adsCustomerIDs(resourceNames),
			"customerCount": len(resourceNames),
		}); err != nil {
			return err
		}
		if len(resourceNames) == 0 {
			return failEmptyExit(c.FailEmpty)
		}
		return nil
	}

	if len(resourceNames) == 0 {
		ui.FromContext(ctx).Err().Println("No directly accessible Google Ads customers")
		return failEmptyExit(c.FailEmpty)
	}

	w, flush := tableWriter(ctx)
	defer flush()
	fmt.Fprintln(w, "CUSTOMER_ID\tRESOURCE_NAME")
	for _, resourceName := range resourceNames {
		fmt.Fprintf(w, "%s\t%s\n", adsCustomerID(resourceName), sanitizeTab(resourceName))
	}

	return nil
}

type AdsFieldsCmd struct {
	AdsConnectionFlags `embed:""`
	ResourceOrField    string `arg:"" name:"resourceOrField" help:"Google Ads resource or field name (for example campaign or metrics.clicks)"`
}

func (c *AdsFieldsCmd) Run(ctx context.Context, flags *RootFlags) error {
	name := strings.TrimSpace(c.ResourceOrField)
	if name == "" {
		return usage("empty resourceOrField")
	}

	client, err := requireAdsClient(ctx, flags, c.AdsConnectionFlags)
	if err != nil {
		return err
	}
	field, err := client.GetField(ctx, name)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, stdoutWriter(ctx), map[string]any{"field": field})
	}

	u := ui.FromContext(ctx)
	return writeResult(ctx, u,
		kv("name", field.Name),
		kv("resource_name", field.ResourceName),
		kv("category", field.Category),
		kv("data_type", field.DataType),
		kv("selectable", field.Selectable),
		kv("filterable", field.Filterable),
		kv("sortable", field.Sortable),
		kv("repeated", field.IsRepeated),
		kv("selectable_with", strings.Join(field.SelectableWith, ",")),
		kv("attribute_resources", strings.Join(field.AttributeResources, ",")),
		kv("metrics", strings.Join(field.Metrics, ",")),
		kv("segments", strings.Join(field.Segments, ",")),
		kv("enum_values", strings.Join(field.EnumValues, ",")),
	)
}

type AdsQueryCmd struct {
	AdsConnectionFlags `embed:""`
	CustomerID         string `arg:"" name:"customerId" help:"Google Ads customer ID (digits or hyphenated)"`
	GAQL               string `name:"gaql" aliases:"query" help:"Google Ads Query Language statement" required:""`
	Page               string `name:"page" aliases:"page-token,cursor" help:"Page token from a previous response"`
	FailEmpty          bool   `name:"fail-empty" aliases:"non-empty,require-results" help:"Exit with code 3 if the query returns no rows"`
}

type AdsAccountCmd struct {
	Create AdsAccountCreateCmd `cmd:"" help:"Create a client account beneath a manager"`
}

type AdsAccountCreateCmd struct {
	AdsConnectionFlags `embed:""`
	ManagerCustomerID  string `arg:"" name:"managerCustomerId" help:"Google Ads manager customer ID (digits or hyphenated)"`
	Name               string `name:"name" help:"Descriptive name for the new client account" required:""`
	CurrencyCode       string `name:"currency-code" help:"Permanent ISO 4217 currency code for the new account" required:""`
	TimeZone           string `name:"time-zone" help:"Permanent IANA time zone for the new account" required:""`
	ValidateOnly       bool   `name:"validate-only" help:"Validate the account creation request without creating the account"`
}

func (c *AdsAccountCreateCmd) Run(ctx context.Context, flags *RootFlags) error {
	managerCustomerID, err := googleapi.NormalizeAdsCustomerID(c.ManagerCustomerID)
	if err != nil {
		return usage(err.Error())
	}
	request := googleapi.AdsCreateCustomerClientRequest{
		CustomerClient: googleapi.AdsCustomerClientCreate{
			DescriptiveName: strings.TrimSpace(c.Name),
			CurrencyCode:    strings.ToUpper(strings.TrimSpace(c.CurrencyCode)),
			TimeZone:        strings.TrimSpace(c.TimeZone),
		},
		ValidateOnly: c.ValidateOnly,
	}
	if request.CustomerClient.DescriptiveName == "" {
		return usage("empty --name")
	}
	if len(request.CustomerClient.CurrencyCode) != 3 {
		return usage("--currency-code must be a three-letter code")
	}
	if request.CustomerClient.TimeZone == "" {
		return usage("empty --time-zone")
	}
	plan := map[string]any{
		"managerCustomerId": managerCustomerID,
		"customerClient":    request.CustomerClient,
		"validateOnly":      request.ValidateOnly,
	}
	if dryRunErr := dryRunExit(ctx, flags, "ads.account.create", plan); dryRunErr != nil {
		return dryRunErr
	}
	if googleapi.ReadOnly(ctx) {
		return fmt.Errorf("%w: Google Ads account creation", googleapi.ErrReadOnly)
	}
	if !c.ValidateOnly {
		if confirmErr := confirmDestructiveChecked(ctx, flags, "create Google Ads client account "+request.CustomerClient.DescriptiveName); confirmErr != nil {
			return confirmErr
		}
	}

	connection := c.AdsConnectionFlags
	connection.LoginCustomerID = managerCustomerID
	client, err := requireAdsClient(ctx, flags, connection)
	if err != nil {
		return err
	}
	response, err := client.CreateCustomerClient(ctx, managerCustomerID, request)
	if err != nil {
		return err
	}

	result := map[string]any{
		"managerCustomerId": managerCustomerID,
		"resourceName":      response.ResourceName,
		"validatedOnly":     c.ValidateOnly,
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, stdoutWriter(ctx), result)
	}
	return writeResult(ctx, ui.FromContext(ctx),
		kv("manager_customer_id", managerCustomerID),
		kv("resource_name", response.ResourceName),
		kv("validated_only", c.ValidateOnly),
	)
}

type AdsConversionCmd struct {
	Create AdsConversionCreateCmd `cmd:"" help:"Create a website conversion action"`
}

type AdsConversionCreateCmd struct {
	AdsConnectionFlags `embed:""`
	CustomerID         string `arg:"" name:"customerId" help:"Google Ads client customer ID (digits or hyphenated)"`
	Name               string `name:"name" help:"Unique conversion action name" required:""`
	Category           string `name:"category" help:"Conversion goal category" enum:"REQUEST_QUOTE,CONTACT" required:""`
	CountingType       string `name:"counting-type" help:"How conversions are counted per ad click" enum:"ONE_PER_CLICK,MANY_PER_CLICK" default:"ONE_PER_CLICK"`
	ValidateOnly       bool   `name:"validate-only" help:"Validate the conversion request without creating the action"`
}

func (c *AdsConversionCreateCmd) Run(ctx context.Context, flags *RootFlags) error {
	customerID, err := googleapi.NormalizeAdsCustomerID(c.CustomerID)
	if err != nil {
		return usage(err.Error())
	}
	action := googleapi.AdsConversionActionCreate{
		Name:           strings.TrimSpace(c.Name),
		Category:       strings.ToUpper(strings.TrimSpace(c.Category)),
		Type:           "WEBPAGE",
		Status:         "ENABLED",
		CountingType:   strings.ToUpper(strings.TrimSpace(c.CountingType)),
		PrimaryForGoal: true,
	}
	if action.Name == "" {
		return usage("empty --name")
	}
	plan := map[string]any{
		"customerId":   customerID,
		"conversion":   action,
		"validateOnly": c.ValidateOnly,
	}
	if dryRunErr := dryRunExit(ctx, flags, "ads.conversion.create", plan); dryRunErr != nil {
		return dryRunErr
	}
	if googleapi.ReadOnly(ctx) {
		return fmt.Errorf("%w: Google Ads conversion creation", googleapi.ErrReadOnly)
	}
	if !c.ValidateOnly {
		if confirmErr := confirmDestructiveChecked(ctx, flags, "create Google Ads conversion action "+action.Name); confirmErr != nil {
			return confirmErr
		}
	}

	client, err := requireAdsClient(ctx, flags, c.AdsConnectionFlags)
	if err != nil {
		return err
	}
	response, err := client.CreateConversionAction(ctx, customerID, action, c.ValidateOnly)
	if err != nil {
		return err
	}
	resourceName := ""
	if len(response.Results) > 0 {
		resourceName = response.Results[0].ResourceName
	}

	result := map[string]any{
		"customerId":    customerID,
		"resourceName":  resourceName,
		"validatedOnly": c.ValidateOnly,
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, stdoutWriter(ctx), result)
	}
	return writeResult(ctx, ui.FromContext(ctx),
		kv("customer_id", customerID),
		kv("resource_name", resourceName),
		kv("validated_only", c.ValidateOnly),
	)
}

func (c *AdsQueryCmd) Run(ctx context.Context, flags *RootFlags) error {
	customerID, err := googleapi.NormalizeAdsCustomerID(c.CustomerID)
	if err != nil {
		return usage(err.Error())
	}
	gaql := strings.TrimSpace(c.GAQL)
	if gaql == "" {
		return usage("empty --gaql")
	}

	client, err := requireAdsClient(ctx, flags, c.AdsConnectionFlags)
	if err != nil {
		return err
	}
	response, err := client.Search(ctx, customerID, googleapi.AdsSearchRequest{
		Query:     gaql,
		PageToken: c.Page,
	})
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		if err := outfmt.WriteJSON(ctx, stdoutWriter(ctx), map[string]any{
			"customerId":               customerID,
			"query":                    gaql,
			"results":                  response.Results,
			"resultCount":              len(response.Results),
			"nextPageToken":            response.NextPageToken,
			"totalResultsCount":        response.TotalResultsCount,
			"fieldMask":                response.FieldMask,
			"summaryRow":               response.SummaryRow,
			"queryResourceConsumption": response.QueryResourceConsumption,
		}); err != nil {
			return err
		}
		if len(response.Results) == 0 {
			return failEmptyExit(c.FailEmpty)
		}
		return nil
	}

	if len(response.Results) == 0 {
		ui.FromContext(ctx).Err().Println("No Google Ads rows")
		return failEmptyExit(c.FailEmpty)
	}

	w, flush := tableWriter(ctx)
	defer flush()
	fmt.Fprintln(w, "ROW\tRESULT_JSON")
	for index, result := range response.Results {
		compact, compactErr := compactAdsJSON(result)
		if compactErr != nil {
			return compactErr
		}
		fmt.Fprintf(w, "%d\t%s\n", index+1, compact)
	}
	printNextPageHint(ui.FromContext(ctx), response.NextPageToken)

	return nil
}

func requireAdsClient(ctx context.Context, flags *RootFlags, connection AdsConnectionFlags) (*googleapi.AdsClient, error) {
	account, err := requireAccount(flags)
	if err != nil {
		return nil, err
	}

	developerToken := firstNonEmptyEnv("GOG_ADS_DEVELOPER_TOKEN", "GOOGLE_ADS_DEVELOPER_TOKEN")
	if developerToken == "" {
		return nil, usage("set GOG_ADS_DEVELOPER_TOKEN or GOOGLE_ADS_DEVELOPER_TOKEN")
	}
	loginCustomerID := strings.TrimSpace(connection.LoginCustomerID)
	if loginCustomerID == "" {
		loginCustomerID = firstNonEmptyEnv("GOG_ADS_LOGIN_CUSTOMER_ID", "GOOGLE_ADS_LOGIN_CUSTOMER_ID")
	}

	client, err := adsService(ctx, account, googleapi.AdsConfig{
		DeveloperToken:  developerToken,
		LoginCustomerID: loginCustomerID,
		APIVersion:      connection.APIVersion,
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func adsCustomerIDs(resourceNames []string) []string {
	ids := make([]string, 0, len(resourceNames))
	for _, resourceName := range resourceNames {
		ids = append(ids, adsCustomerID(resourceName))
	}
	return ids
}

func adsCustomerID(resourceName string) string {
	return strings.TrimPrefix(strings.TrimSpace(resourceName), "customers/")
}

func compactAdsJSON(value json.RawMessage) (string, error) {
	if len(value) == 0 {
		return "null", nil
	}
	var output bytes.Buffer
	if err := json.Compact(&output, value); err != nil {
		return "", fmt.Errorf("compact Google Ads result: %w", err)
	}
	return output.String(), nil
}
