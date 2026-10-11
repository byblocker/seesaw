package report

import (
	"fmt"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/trading"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

func Orders(plan trading.RebalancePlan, buyingPower decimal.Decimal, rates *config.TaxSettings) string {
	var output strings.Builder
	output.WriteString("\nSuggested Orders\n\n")

	if len(plan.Buys) == 0 && len(plan.Sells) == 0 {
		output.WriteString("No trades suggested.\n")
		return output.String()
	}

	plan = sortOrders(plan)
	sells := formatSells(plan.Sells, rates, nil)
	buyRows, cost := formatBuys(plan.Buys)
	headers := []string{"Ticker", "Action", "Acquired", "Shares", "Price", "Cost", "P/L"}
	if rates != nil {
		headers = append(headers, "Est. tax impact")
		for i := range buyRows {
			buyRows[i] = append(buyRows[i], "—")
		}
	}
	writeTable(&output, headers, append(sells.rows, buyRows...), 3)

	remainingCash := buyingPower.Add(sells.proceeds).Sub(cost)

	writeSummaries(&output, orderSummaries(sells, remainingCash))
	return output.String()
}

func sortOrders(plan trading.RebalancePlan) trading.RebalancePlan {
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

func orderSummaries(sells formattedSells, remainingCash decimal.Decimal) [][2]string {
	summaries := [][2]string{
		{"Total P/L:", money(sells.realizedPL)},
		{"Remaining balance:", money(remainingCash)},
	}

	if sells.showTax {
		summaries = append(summaries,
			[2]string{"Estimated tax on gains:", money(sells.tax)},
			[2]string{"Potential tax relief:", money(sells.relief)},
		)
	}

	return summaries
}

var summaryRule = [2]string{}

// Align every group's labels and amounts together, separating groups with a blank line.
func writeSummaries(output *strings.Builder, groups ...[][2]string) {
	var labelWidth, amountWidth int
	for _, summaries := range groups {
		for _, summary := range summaries {
			labelWidth = max(labelWidth, len(summary[0]))
			amountWidth = max(amountWidth, len(summary[1]))
		}
	}

	for i, summaries := range groups {
		if i > 0 {
			output.WriteByte('\n')
		}
		for _, summary := range summaries {
			if summary == summaryRule {
				output.WriteString(strings.Repeat("_", labelWidth+1+amountWidth))
				output.WriteByte('\n')
				continue
			}
			fmt.Fprintf(output, "%-*s %*s\n", labelWidth, summary[0], amountWidth, summary[1])
		}
	}
}
