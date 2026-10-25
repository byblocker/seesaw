package cmd

import (
	"fmt"
	"io"
	"log"
	"seesaw/internal/report"
	"seesaw/internal/utils"
	"seesaw/pkg/broker"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
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
	liquidatePortfolio.Flags().IntVar(&liquidationSearchLimit, "search-limit", 100_000, "maximum search nodes before reporting an incomplete search")
	for _, flag := range []string{"portfolio", "amount"} {
		if err := liquidatePortfolio.MarkFlagRequired(flag); err != nil {
			panic(err)
		}
	}
}

func liquidate(cmd *cobra.Command, args []string) error {
	amount, err := decimal.NewFromString(strings.TrimSpace(liquidationAmount))
	if err != nil {
		return fmt.Errorf("--amount must be a decimal")
	}

	plan, err := loadPlan(liquidationPlanName)
	if err != nil {
		return err
	}
	log.Printf("Requested liquidation of %s for %s", plan.AccountLabel(), utils.FormatMoney(amount))
	user, err := config.LoadUser(userConfigPath)
	if err != nil {
		return err
	}

	request := models.LiquidationRequest{
		Amount:      amount,
		Settings:    user.Liquidation,
		TaxRates:    taxRates(plan, user),
		SearchLimit: liquidationSearchLimit,
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

	selection, err := trading.Liquidate(cmd.Context(), *snapshot, request)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(cmd.OutOrStdout(), report.Liquidation(*selection, *snapshot)); err != nil {
		return fmt.Errorf("write liquidation summary: %w", err)
	}
	return nil
}
