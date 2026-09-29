package cmd

import (
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg"

	"github.com/spf13/cobra"
)

var rebalancePortfolio = &cobra.Command{
	Use:   "rebalance",
	Short: "Rebalance portfolio according to portfolio plan",
	RunE:  rebalance,
}

func init() {
	rootCmd.AddCommand(rebalancePortfolio)
}

func rebalance(cmd *cobra.Command, args []string) error {
	log.Print("Starting rebalance . . .")
	ctx := cmd.Context()

	client, err := pkg.ConnectFromENV(ctx, plan.Broker)
	if err != nil {
		return err
	}

	snapshot, err := pkg.SnapshotPortfolio(ctx, client, plan)
	if err != nil {
		return err
	}

	orders, err := pkg.Rebalance(ctx, *snapshot, client, plan.Settings)
	if err != nil {
		return err
	}

	utils.DebugYAML(orders)
	return nil
}
