package report

import (
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"strings"

	"github.com/shopspring/decimal"
)

func Orders(plan models.RebalancePlan, rates *config.TaxSettings) string {
	var output strings.Builder
	output.WriteString("\nSuggested Orders\n\n")

	if len(plan.Buys) == 0 && len(plan.Sells) == 0 {
		output.WriteString("No trades suggested.\n")
		return output.String()
	}

	sells := formatSells(plan.Sells, rates)
	buyRows := formatBuys(plan.Buys, rates)
	headers := []string{"Ticker", "Action", "Acquired", "Shares", "Price", "Cost", "P/L"}
	if rates != nil {
		headers = append(headers, "Est. tax impact")
	}
	writeTable(&output, headers, append(sells.rows, buyRows...), 3)

	writeSummaries(&output, orderSummaries(sells, plan.RemainingBalance))
	return output.String()
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
