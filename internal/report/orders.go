package report

import (
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"strings"

	"github.com/shopspring/decimal"
)

func Orders(plan models.RebalancePlan, estimated portfolio.Snapshot, rates *config.TaxSettings) string {
	var output strings.Builder
	output.WriteString("\nSuggested Orders\n\n")

	if len(plan.Buys) == 0 && len(plan.Sells) == 0 {
		output.WriteString("No trades suggested.\n")
		return output.String()
	}

	sells := formatSells(plan.Sells, rates)
	buyRows := formatBuys(plan.Buys, rates)
	writeOrderTable(&output, "Cost", append(sells.rows, buyRows...), rates)

	writeSummaries(&output, orderSummaries(sells, plan.RemainingBalance))
	writeEstimatedPortfolio(&output, estimated)
	return output.String()
}

func orderSummaries(sells formattedSells, remainingCash decimal.Decimal) [][2]string {
	summaries := [][2]string{
		{"Total P/L:", utils.FormatMoney(sells.realizedPL)},
		{"Remaining balance:", utils.FormatMoney(remainingCash)},
	}

	if sells.showTax {
		summaries = append(summaries,
			[2]string{"Estimated tax on gains:", utils.FormatMoney(sells.tax)},
			[2]string{"Potential tax relief:", utils.FormatMoney(sells.relief)},
		)
	}

	return summaries
}
