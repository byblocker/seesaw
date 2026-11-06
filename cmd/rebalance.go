package cmd

import (
	"log"
	"seesaw/internal/broker"
	"seesaw/pkg/portfolio"
	"seesaw/pkg/rebalancer"

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
	taxRates, err := loadTaxRates(plan)
	if err != nil {
		return err
	}

	client, err := broker.Connect(cmd.Context(), plan.Broker)
	if err != nil {
		return err
	}

	snapshot, err := portfolio.TakeSnapshot(cmd.Context(), client, plan, taxRates)
	if err != nil {
		return err
	}
	if err := writeSnapshot(cmd, snapshot); err != nil {
		return err
	}

	proposal, err := rebalancer.Plan(snapshot, plan.Settings, forceRebalance)
	if err != nil {
		return err
	}

	return writeProposal(cmd, snapshot, proposal)
}
