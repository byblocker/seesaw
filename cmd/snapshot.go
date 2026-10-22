package cmd

import (
	"fmt"
	"io"
	"seesaw/internal/report"
	"seesaw/pkg/broker"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/spf13/cobra"
)

var snapshotPlanName string

var snapshotPortfolio = &cobra.Command{
	Use:   "snapshot",
	Short: "Show the account's holdings against its portfolio plan",
	Args:  cobra.NoArgs,
	RunE:  showSnapshot,
}

func init() {
	rootCmd.AddCommand(snapshotPortfolio)
	snapshotPortfolio.Flags().StringVar(&snapshotPlanName, "portfolio", "", "name of portfolio plan")
	if err := snapshotPortfolio.MarkFlagRequired("portfolio"); err != nil {
		panic(err)
	}
}

func showSnapshot(cmd *cobra.Command, args []string) error {
	plan, err := loadPlan(snapshotPlanName)
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

func takeSnapshot(cmd *cobra.Command, client models.BrokerClient, plan *config.Plan) (*portfolio.Snapshot, error) {
	snapshot, err := portfolio.TakeSnapshot(cmd.Context(), client, plan)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(cmd.OutOrStdout(), report.Snapshot(*snapshot)); err != nil {
		return nil, fmt.Errorf("write snapshot summary: %w", err)
	}
	return snapshot, nil
}
