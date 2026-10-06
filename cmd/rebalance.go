package cmd

import (
	"fmt"
	"io"
	"log"
	"path/filepath"
	"seesaw/internal/report"
	"seesaw/pkg/broker"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"seesaw/pkg/trading"

	"github.com/spf13/cobra"
)

const portfolioDir = "./data/portfolios"

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
	ctx := cmd.Context()

	pathToPlan := filepath.Join(portfolioDir, planName+".yaml")
	plan, err := config.LoadPlan(pathToPlan)
	if err != nil {
		return err
	}

	var rates *models.TaxSettings
	if *plan.IsTaxable {
		user, err := config.LoadUser("./data/user.yaml")
		if err != nil {
			return err
		}
		rates = &user.Tax
	}

	client, err := broker.Connect(ctx, plan.Broker)
	if err != nil {
		return err
	}

	snapshot, err := portfolio.Snapshot(ctx, client, plan)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(cmd.OutOrStdout(), report.Snapshot(*snapshot)); err != nil {
		return fmt.Errorf("write snapshot summary: %w", err)
	}

	orders, err := trading.Rebalance(ctx, *snapshot, client, plan.Rebalance)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(cmd.OutOrStdout(), report.Orders(*orders, snapshot.Portfolio.Balances.BuyingPower, rates)); err != nil {
		return fmt.Errorf("write proposed orders: %w", err)
	}
	return nil
}
