package cmd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"seesaw/internal/report"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"seesaw/pkg/registry"
	"seesaw/pkg/secrets/keyring"

	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:     "snapshot <portfolio>",
	Short:   "Show the account's holdings against its portfolio plan",
	Example: "  seesaw snapshot growth",
	Args:    requirePlanName,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _, err := snapshotAccount(cmd, args[0])
		return err
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}

// Snapshot the named plan's account and write it.
func snapshotAccount(cmd *cobra.Command, planName string) (*config.Plan, portfolio.Snapshot, error) {
	plan, err := loadPlan(planName)
	if err != nil {
		return nil, portfolio.Snapshot{}, err
	}
	log.Printf("Running %s for %s with %s portfolio", cmd.Name(), plan.AccountLabel(), planName)
	taxRates, err := loadTaxRates(plan)
	if err != nil {
		return nil, portfolio.Snapshot{}, err
	}

	selectedBroker, err := registry.FindBroker(plan.Broker, keyring.Store{})
	if err != nil {
		return nil, portfolio.Snapshot{}, err
	}
	client, err := selectedBroker.Connect(cmd.Context())
	if errors.Is(err, models.ErrBrokerNotConfigured) {
		return nil, portfolio.Snapshot{}, fmt.Errorf("%w; run seesaw connect --broker %s", err, plan.Broker)
	}
	if err != nil {
		return nil, portfolio.Snapshot{}, err
	}

	snapshot, err := portfolio.TakeSnapshot(cmd.Context(), client, plan, taxRates)
	if err != nil {
		return nil, portfolio.Snapshot{}, err
	}
	if _, err := io.WriteString(cmd.OutOrStdout(), report.Snapshot(snapshot)); err != nil {
		return nil, portfolio.Snapshot{}, fmt.Errorf("write snapshot summary: %w", err)
	}
	return plan, snapshot, nil
}

// Snapshot the named plan's account, then write the proposed orders and the portfolio they would leave.
func proposeFor(
	cmd *cobra.Command,
	planName string,
	propose func(plan *config.Plan, snapshot portfolio.Snapshot) (portfolio.Proposal, error),
) error {
	plan, snapshot, err := snapshotAccount(cmd, planName)
	if err != nil {
		return err
	}
	proposal, err := propose(plan, snapshot)
	if err != nil {
		return err
	}
	text := report.Proposal(proposal, snapshot.Simulate(proposal))
	if _, err := io.WriteString(cmd.OutOrStdout(), text); err != nil {
		return fmt.Errorf("write proposed orders: %w", err)
	}
	return nil
}
