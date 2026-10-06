package report

import (
	"fmt"
	"seesaw/pkg"
	"seesaw/pkg/models"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

func Orders(plan pkg.RebalancePlan, buyingPower decimal.Decimal, rates *models.TaxSettings) string {
	var output strings.Builder
	output.WriteString("\nSuggested Orders\n\n")

	if len(plan.Buys) == 0 && len(plan.Sells) == 0 {
		output.WriteString("No trades suggested.\n")
		return output.String()
	}

	plan = sortOrders(plan)
	sellRows, proceeds, realizedPL, _ := formatSells(plan.Sells, rates, nil)
	buyRows, cost := formatBuys(plan.Buys)
	headers := []string{"Ticker", "Action", "Acquired", "Shares", "Price", "Cost", "P/L"}
	if rates != nil {
		headers = append(headers, "Est. tax impact")
		for i := range buyRows {
			buyRows[i] = append(buyRows[i], "—")
		}
	}
	writeTable(&output, headers, append(sellRows, buyRows...), 3)

	remainingCash := buyingPower.Add(proceeds).Sub(cost)

	writeSummaries(&output, orderSummaries(realizedPL, remainingCash, plan.Sells, rates))
	if rates != nil && len(plan.Sells) > 0 {
		output.WriteString("\nNegative tax impact is potential relief, assuming losses offset gains of the same holding period.\n")
		output.WriteString("Estimates exclude annual netting, carryovers, and wash-sale adjustments; they are not tax due.\n")
	}
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

func orderSummaries(realizedPL, remainingCash decimal.Decimal, sells []models.SellProposal, rates *models.TaxSettings) [][2]string {
	summaries := [][2]string{
		{"Total P/L:", money(realizedPL)},
		{"Remaining balance:", money(remainingCash)},
	}

	if rates != nil && len(sells) > 0 {
		tax, relief := decimal.Zero, decimal.Zero
		for _, sale := range sells {
			for _, lot := range sale.Lots {
				impact := lot.EstimatedTaxImpact(*rates)
				if impact.IsNegative() {
					relief = relief.Sub(impact)
				} else {
					tax = tax.Add(impact)
				}
			}
		}
		summaries = append(summaries,
			[2]string{"Estimated tax on gains:", money(tax)},
			[2]string{"Potential tax relief:", money(relief)},
		)
	}

	return summaries
}

func writeSummaries(output *strings.Builder, summaries [][2]string) {
	var labelWidth, amountWidth int
	for _, summary := range summaries {
		labelWidth = max(labelWidth, len(summary[0]))
		amountWidth = max(amountWidth, len(summary[1]))
	}

	for _, summary := range summaries {
		fmt.Fprintf(output, "%-*s %*s\n", labelWidth, summary[0], amountWidth, summary[1])
	}
}
