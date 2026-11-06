package cmd

import (
	"fmt"
	"log"
	"regexp"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/liquidation"
	"seesaw/pkg/portfolio"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

var liquidationAmount string

var liquidateCmd = &cobra.Command{
	Use:     "liquidate <portfolio>",
	Short:   "Plan whole-share sales to fund a withdrawal",
	Example: "  seesaw liquidate growth --amount 1000\n  seesaw liquidate growth -a '$1,000.50'",
	Args:    requirePlanName,
	RunE: func(cmd *cobra.Command, args []string) error {
		amount, err := parseAmount(liquidationAmount)
		if err != nil {
			return err
		}
		log.Printf("Requested withdrawal of %s", utils.FormatMoney(amount))

		return proposeFor(cmd, args[0], func(plan *config.Plan, snapshot portfolio.Snapshot) (portfolio.Proposal, error) {
			return liquidation.PlanSales(snapshot, plan.Settings, amount)
		})
	},
}

func init() {
	rootCmd.AddCommand(liquidateCmd)
	liquidateCmd.Flags().StringVarP(&liquidationAmount, "amount", "a", "", "withdrawal amount in account currency")
	if err := liquidateCmd.MarkFlagRequired("amount"); err != nil {
		panic(err)
	}
}

var amountPattern = regexp.MustCompile(`^\$?(\d+|\d{1,3}(,\d{3})+)(\.\d+)?$`)

func parseAmount(text string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(text)
	if !amountPattern.MatchString(trimmed) {
		return decimal.Zero, fmt.Errorf("--amount must be a number, such as 1000 or '$1,000.50'; got %q", text)
	}
	cleaned := strings.ReplaceAll(strings.TrimPrefix(trimmed, "$"), ",", "")
	amount, err := decimal.NewFromString(cleaned)
	if err != nil {
		return decimal.Zero, err
	}
	if !amount.IsPositive() {
		return decimal.Zero, fmt.Errorf("withdrawal must be positive")
	}
	return amount, nil
}
