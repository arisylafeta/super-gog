package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/steipete/gogcli/internal/googleapi"
	"github.com/steipete/gogcli/internal/outfmt"
)

type AdsMutateCmd struct {
	AdsConnectionFlags `embed:""`
	CustomerID         string `arg:"" name:"customerId" help:"Google Ads client customer ID"`
	Service            string `name:"service" required:"" help:"REST service: googleAds, campaigns, campaignBudgets, campaignCriteria, adGroups, adGroupCriteria, adGroupAds, ads, experiments, experimentArms, customConversionGoals, conversionGoalCampaignConfigs"`
	BodyFile           string `name:"body-file" required:"" help:"JSON mutation request file; validated atomically by default"`
	Apply              bool   `name:"apply" help:"Apply the request instead of validation only; requires confirmation"`
}

func (c *AdsMutateCmd) Run(ctx context.Context, flags *RootFlags) error {
	id, err := googleapi.NormalizeAdsCustomerID(c.CustomerID)
	if err != nil {
		return usage(err.Error())
	}

	body, err := os.ReadFile(c.BodyFile)
	if err != nil {
		return fmt.Errorf("read mutation file: %w", err)
	}

	request, err := googleapi.PrepareAdsMutation(c.Service, body, c.Apply)
	if err != nil {
		return usage(err.Error())
	}

	plan := map[string]any{"customerId": id, "service": c.Service, "request": request}

	if dryRunErr := dryRunExit(ctx, flags, "ads.mutate", plan); dryRunErr != nil {
		return dryRunErr
	}

	if googleapi.ReadOnly(ctx) {
		return fmt.Errorf("%w: Google Ads mutation", googleapi.ErrReadOnly)
	}

	if c.Apply {
		if confirmErr := confirmDestructiveChecked(ctx, flags, "apply Google Ads "+c.Service+" mutation for customer "+id); confirmErr != nil {
			return confirmErr
		}
	}

	client, err := requireAdsClient(ctx, flags, c.AdsConnectionFlags)
	if err != nil {
		return err
	}

	response, err := client.Mutate(ctx, id, c.Service, body, c.Apply)
	if err != nil {
		return err
	}

	return outfmt.WriteJSON(ctx, stdoutWriter(ctx), map[string]any{"customerId": id, "service": c.Service, "validatedOnly": !c.Apply, "response": response})
}

type AdsExperimentCmd struct {
	Schedule AdsExperimentScheduleCmd `cmd:"" help:"Validate or schedule an existing experiment (may start serving)"`
}

type AdsExperimentScheduleCmd struct {
	AdsConnectionFlags `embed:""`
	CustomerID         string `arg:"" name:"customerId" help:"Google Ads client customer ID"`
	ExperimentID       string `arg:"" name:"experimentId" help:"Existing experiment ID"`
	Apply              bool   `name:"apply" help:"Schedule rather than validate; may start serving and spending"`
}

func (c *AdsExperimentScheduleCmd) Run(ctx context.Context, flags *RootFlags) error {
	id, err := googleapi.NormalizeAdsCustomerID(c.CustomerID)
	if err != nil {
		return usage(err.Error())
	}

	experiment, err := googleapi.NormalizeAdsCustomerID(c.ExperimentID)
	if err != nil {
		return usage(err.Error())
	}

	plan := map[string]any{"customerId": id, "experimentId": experiment, "validateOnly": !c.Apply}

	if dryRunErr := dryRunExit(ctx, flags, "ads.experiment.schedule", plan); dryRunErr != nil {
		return dryRunErr
	}

	if googleapi.ReadOnly(ctx) {
		return fmt.Errorf("%w: schedule Google Ads experiment", googleapi.ErrReadOnly)
	}

	if c.Apply {
		if confirmErr := confirmDestructiveChecked(ctx, flags, "schedule Google Ads experiment "+experiment+" (may start serving and spending)"); confirmErr != nil {
			return confirmErr
		}
	}

	client, err := requireAdsClient(ctx, flags, c.AdsConnectionFlags)
	if err != nil {
		return err
	}

	response, err := client.ScheduleExperiment(ctx, id, experiment, c.Apply)
	if err != nil {
		return err
	}

	return outfmt.WriteJSON(ctx, stdoutWriter(ctx), map[string]any{"customerId": id, "experimentId": experiment, "validatedOnly": !c.Apply, "response": response})
}
