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
	"strings"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

var (
	liquidationPlanName    string
	liquidationAmount      string
	liquidationSearchLimit int
)

var liquidatePortfolio = &cobra.Command{
	Use:   "liquidate",
	Short: "Plan whole-share sales to fund a withdrawal",
	Args:  cobra.NoArgs,
	RunE:  liquidate,
}

func init() {
	rootCmd.AddCommand(liquidatePortfolio)
	liquidatePortfolio.Flags().StringVar(&liquidationPlanName, "portfolio", "", "name of portfolio plan")
	liquidatePortfolio.Flags().StringVar(&liquidationAmount, "amount", "", "withdrawal amount in account currency")
	liquidatePortfolio.Flags().IntVar(&liquidationSearchLimit, "search-limit", 1_000_000, "maximum search nodes before reporting an incomplete search")
	for _, flag := range []string{"portfolio", "amount"} {
		if err := liquidatePortfolio.MarkFlagRequired(flag); err != nil {
			panic(err)
		}
	}
}

func liquidate(cmd *cobra.Command, args []string) error {
	amount, err := decimal.NewFromString(strings.TrimSpace(liquidationAmount))
	if err != nil || !amount.IsPositive() {
		return fmt.Errorf("--amount must be a positive decimal")
	}
	if liquidationSearchLimit <= 0 {
		return fmt.Errorf("--search-limit must be positive")
	}
	if strings.TrimSpace(liquidationPlanName) == "" {
		return fmt.Errorf("--portfolio must not be empty")
	}

	log.Print("Starting liquidation planning . . .")
	ctx := cmd.Context()
	plan, err := config.LoadPlan(filepath.Join(portfolioDir, liquidationPlanName+".yaml"))
	if err != nil {
		return err
	}

	var rates *config.TaxSettings
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

	snapshot, err := portfolio.TakeSnapshot(ctx, client, plan)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(cmd.OutOrStdout(), report.Snapshot(*snapshot)); err != nil {
		return fmt.Errorf("write snapshot summary: %w", err)
	}

	selection, err := trading.Liquidate(ctx, *snapshot, client, models.LiquidationRequest{
		Amount:      amount,
		Settings:    plan.Liquidation,
		TaxRates:    rates,
		SearchLimit: liquidationSearchLimit,
	})
	if err != nil {
		return err
	}

	if _, err := io.WriteString(cmd.OutOrStdout(), report.Liquidation(*selection, snapshot.Portfolio.RecentTx)); err != nil {
		return fmt.Errorf("write liquidation summary: %w", err)
	}
	return nil
}
