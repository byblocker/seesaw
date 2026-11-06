package cmd

import (
	"fmt"
	"log"
	"regexp"
	"seesaw/internal/broker"
	"seesaw/internal/utils"
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
	RunE:    liquidate,
}

func init() {
	rootCmd.AddCommand(liquidateCmd)
	liquidateCmd.Flags().StringVarP(&liquidationAmount, "amount", "a", "", "withdrawal amount in account currency")
	if err := liquidateCmd.MarkFlagRequired("amount"); err != nil {
		panic(err)
	}
}

func liquidate(cmd *cobra.Command, args []string) error {
	amount, err := parseAmount(liquidationAmount)
	if err != nil {
		return err
	}

	plan, err := loadPlan(args[0])
	if err != nil {
		return err
	}
	log.Printf("Requested liquidation of %s for %s", plan.AccountLabel(), utils.FormatMoney(amount))
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

	sells, err := liquidation.PlanSales(snapshot, plan.Settings, amount)
	if err != nil {
		return err
	}

	return writeProposal(cmd, snapshot, portfolio.Proposal{Sells: sells, Withdrawal: amount})
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
