package cmd

import (
	"fmt"
	"log"
	"seesaw/internal/config"
	"seesaw/internal/utils"
	"seesaw/pkg"
	"seesaw/pkg/etrade"

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

	env, err := config.LoadEnv()
	if err != nil {
		return fmt.Errorf("failed to load .env: %w", err)
	}

	client, err := etrade.NewClient(
		ctx,
		env.ETradeApiKey,
		env.ETradeApiSecret,
		env.ETradeAPIBaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to connect to ETrade: %w", err)
	}

	snapshot, err := pkg.SnapshotPortfolio(ctx, client, portfolioPlan, 0)
	if err != nil {
		return err
	}

	orders, err := pkg.Rebalance(ctx, *snapshot, client)
	if err != nil {
		return err
	}

	utils.DebugYAML(orders)
	return nil
}
