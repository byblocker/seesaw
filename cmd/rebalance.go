package cmd

import (
	"fmt"
	"io"
	"log"
	"seesaw/internal/report"
	"seesaw/pkg/config"
	"seesaw/pkg/trading"

	"github.com/spf13/cobra"
)

var planName string

var rebalancePortfolio = &cobra.Command{
	Use:   "rebalance",
	Short: "Rebalance portfolio according to portfolio plan",
	RunE:  rebalance,
}

func init() {
	rootCmd.AddCommand(rebalancePortfolio)
	rebalancePortfolio.Flags().StringVar(&planName, "portfolio", "", "name of portfolio plan")
	if err := rebalancePortfolio.MarkFlagRequired("portfolio"); err != nil {
		panic(err)
	}
}

func rebalance(cmd *cobra.Command, args []string) error {
	log.Print("Starting rebalance . . .")
	plan, err := loadPlan(planName)
	if err != nil {
		return err
	}
	user, err := config.LoadUser(userConfigPath)
	if err != nil {
		return err
	}

	client, snapshot, err := takeSnapshot(cmd, plan)
	if err != nil {
		return err
	}

	orders, err := trading.Rebalance(cmd.Context(), *snapshot, client, user.Rebalance)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(cmd.OutOrStdout(), report.Orders(*orders, taxRates(plan, user))); err != nil {
		return fmt.Errorf("write proposed orders: %w", err)
	}
	return nil
}
