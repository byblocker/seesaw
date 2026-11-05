package cmd

import (
	"fmt"
	"io"
	"seesaw/internal/report"
	"seesaw/pkg/broker"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:     "snapshot <portfolio>",
	Short:   "Show the account's holdings against its portfolio plan",
	Example: "  seesaw snapshot growth",
	Args:    requirePlanName,
	RunE:    showSnapshot,
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}

func showSnapshot(cmd *cobra.Command, args []string) error {
	plan, err := loadPlan(args[0])
	if err != nil {
		return err
	}

	client, err := broker.Connect(cmd.Context(), plan.Broker)
	if err != nil {
		return err
	}

	_, err = takeSnapshot(cmd, client, plan)
	return err
}

func takeSnapshot(cmd *cobra.Command, client models.BrokerClient, plan *config.Plan) (portfolio.Snapshot, error) {
	snapshot, err := portfolio.TakeSnapshot(cmd.Context(), client, plan)
	if err != nil {
		return portfolio.Snapshot{}, err
	}
	if _, err := io.WriteString(cmd.OutOrStdout(), report.Snapshot(snapshot)); err != nil {
		return portfolio.Snapshot{}, fmt.Errorf("write snapshot summary: %w", err)
	}
	return snapshot, nil
}

// Write the orders and the portfolio they would leave after the withdrawal.
func writeProposal(
	cmd *cobra.Command,
	snapshot portfolio.Snapshot,
	sells []portfolio.SellProposal,
	buys []portfolio.Buy,
	withdrawal decimal.Decimal,
	taxRates *config.TaxSettings,
) error {
	estimated := snapshot.Simulate(sells, buys, withdrawal)
	proposal := report.Proposal(sells, buys, estimated, taxRates)
	if _, err := io.WriteString(cmd.OutOrStdout(), proposal); err != nil {
		return fmt.Errorf("write proposed orders: %w", err)
	}
	return nil
}
