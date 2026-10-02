package report

import (
	"fmt"
	"seesaw/pkg"
	"seesaw/pkg/models"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

func Orders(plan pkg.RebalancePlan, buyingPower decimal.Decimal) string {
	var output strings.Builder
	output.WriteString("\nSuggested Orders\n\n")

	if len(plan.Buys) == 0 && len(plan.Sells) == 0 {
		output.WriteString("No trades suggested.\n")
		return output.String()
	}

	plan = sortOrders(plan)
	sellRows, proceeds, realizedPL := formatSells(plan.Sells)
	buyRows, cost := formatBuys(plan.Buys)
	writeTable(&output,
		[]string{"Ticker", "Action", "Acquired", "Shares", "Price", "Cost", "P/L"},
		append(sellRows, buyRows...), 3)

	remainingCash := buyingPower.Add(proceeds).Sub(cost)

	writeOrderTotals(&output, realizedPL, remainingCash)
	return output.String()
}

func sortOrders(plan pkg.RebalancePlan) pkg.RebalancePlan {
	plan.Buys = slices.Clone(plan.Buys)
	slices.SortFunc(plan.Buys, func(left, right models.Buy) int {
		return strings.Compare(left.Ticker, right.Ticker)
	})

	plan.Sells = slices.Clone(plan.Sells)
	slices.SortFunc(plan.Sells, func(left, right models.SellProposal) int {
		return strings.Compare(left.Ticker, right.Ticker)
	})
	return plan
}

func writeOrderTotals(output *strings.Builder, realizedPL, remainingCash decimal.Decimal) {
	summaries := [][2]string{
		{"Total P/L:", money(realizedPL)},
		{"Remaining balance:", money(remainingCash)},
	}

	var labelWidth, amountWidth int
	for _, summary := range summaries {
		labelWidth = max(labelWidth, len(summary[0]))
		amountWidth = max(amountWidth, len(summary[1]))
	}

	for _, summary := range summaries {
		fmt.Fprintf(output, "%-*s %*s\n", labelWidth, summary[0], amountWidth, summary[1])
	}
}
