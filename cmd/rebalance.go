package cmd

import (
	"fmt"
	"io"
	"log"
	"seesaw/internal/report"
	"seesaw/pkg/broker"
	"seesaw/pkg/rebalancer"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

var forceRebalance bool

var rebalanceCmd = &cobra.Command{
	Use:     "rebalance <portfolio>",
	Short:   "Rebalance portfolio according to portfolio plan",
	Example: "  seesaw rebalance growth\n  seesaw rebalance retirement --force",
	Args:    requirePlanName,
	RunE:    rebalance,
}

func init() {
	rootCmd.AddCommand(rebalanceCmd)
	rebalanceCmd.Flags().BoolVarP(&forceRebalance, "force", "f", false, "sell overweight assets and holdings outside the plan to fund underweight assets")
}

func rebalance(cmd *cobra.Command, args []string) error {
	planName := args[0]
	plan, err := loadPlan(planName)
	if err != nil {
		return err
	}
	log.Printf("Requested rebalance of %s with %s portfolio", plan.AccountLabel(), planName)
	user, err := loadUser()
	if err != nil {
		return err
	}
	taxRates, err := user.TaxRates(plan.IsTaxable)
	if err != nil {
		return err
	}

	request := rebalancer.Request{
		Settings: plan.Settings,
		TaxRates: taxRates,
		Force:    forceRebalance,
	}
	// Reject a bad request before the broker login, not after.
	if err := request.Validate(); err != nil {
		return err
	}

	client, err := broker.Connect(cmd.Context(), plan.Broker)
	if err != nil {
		return err
	}

	snapshot, err := takeSnapshot(cmd, client, plan)
	if err != nil {
		return err
	}

	sells, buys, err := rebalancer.Plan(snapshot, request, client.Settings())
	if err != nil {
		return err
	}

	estimated := snapshot.Simulate(sells, buys, decimal.Zero)
	proposal := report.Proposal(sells, buys, estimated, request.TaxRates)

	if _, err := io.WriteString(cmd.OutOrStdout(), proposal); err != nil {
		return fmt.Errorf("write proposed orders: %w", err)
	}
	return nil
}
