package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"seesaw/internal/report"
	"seesaw/pkg"

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
	file, err := os.Open(pathToPlan)
	if err != nil {
		return fmt.Errorf("opening portfolio plan file %s: %w", pathToPlan, err)
	}
	defer file.Close()

	plan, err := pkg.ParsePortfolioPlan(file)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	client, err := pkg.Connect(ctx, plan.Broker)
	if err != nil {
		return err
	}

	snapshot, err := pkg.SnapshotPortfolio(ctx, client, plan)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(cmd.OutOrStdout(), report.Snapshot(*snapshot)); err != nil {
		return fmt.Errorf("write snapshot summary: %w", err)
	}

	orders, err := pkg.Rebalance(ctx, *snapshot, client, plan.Rebalance)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(cmd.OutOrStdout(), report.Orders(*orders, snapshot.BuyingPower)); err != nil {
		return fmt.Errorf("write proposed orders: %w", err)
	}
	return nil
}
