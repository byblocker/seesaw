package cmd

import (
	"seesaw/pkg/config"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		return proposeFor(cmd, args[0], func(plan *config.Plan, snapshot portfolio.Snapshot) (portfolio.Proposal, error) {
			return rebalancer.Plan(snapshot, plan.Settings, forceRebalance)
		})
	},
}

func init() {
	rootCmd.AddCommand(rebalanceCmd)
	rebalanceCmd.Flags().BoolVarP(&forceRebalance, "force", "f", false, "sell overweight assets and holdings outside the plan to fund underweight assets")
}
