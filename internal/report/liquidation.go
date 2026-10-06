package report

import (
	"seesaw/pkg/models"
	"seesaw/pkg/trading"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

func Liquidation(plan trading.LiquidationPlan, rates *models.TaxSettings, recentTx models.RecentTxByTicker) string {
	var output strings.Builder
	output.WriteString("\nSuggested Liquidation\n\n")

	sells := slices.Clone(plan.Sells)
	slices.SortFunc(sells, func(a, b models.SellProposal) int { return strings.Compare(a.Ticker, b.Ticker) })
	formatted := formatSells(sells, rates, recentTx)
	if len(formatted.rows) == 0 {
		output.WriteString("No sales required.\n\n")
	} else {
		headers := []string{"Ticker", "Action", "Acquired", "Shares", "Price", "Proceeds", "P/L"}
		if rates != nil {
			headers = append(headers, "Est. tax impact")
		}
		writeTable(&output, headers, formatted.rows, 3)
	}

	availableNow := decimal.Min(plan.Withdrawal, plan.AvailableCash)
	summaries := [][2]string{
		{"Requested withdrawal:", money(plan.Withdrawal)},
		{"Available now for this withdrawal:", money(availableNow)},
		{"Estimated sale proceeds:", money(formatted.proceeds)},
	}
	// The planner's TotalProceeds includes existing cash less the withdrawal.
	summaries = append(summaries, orderSummaries(formatted, plan.TotalProceeds)...)
	writeSummaries(&output, summaries)

	if availableNow.LessThan(plan.Withdrawal) {
		output.WriteString("\nThe full withdrawal is not available now; cash availability depends on settlement and broker restrictions.\n")
	}
	if formatted.washSale {
		output.WriteString("\n[WS] Loss sale with a recent purchase of the same ticker; some or all tax relief may be disallowed.\n")
		output.WriteString("This check does not cover future purchases or all substantially identical securities.\n")
	}
	if formatted.showTax {
		output.WriteString("\nNegative tax impact is potential relief, assuming losses offset gains of the same holding period.\n")
		output.WriteString("Estimates exclude annual netting, carryovers, and wash-sale adjustments; they are not tax due.\n")
		output.WriteString("The plan does not reserve cash for tax payments.\n")
	}
	return output.String()
}
