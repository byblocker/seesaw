package report

import (
	"fmt"
	"seesaw/pkg/models"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

func Liquidation(selection models.LiquidationSelection, recentTx models.RecentTxByTicker) string {
	plan := selection.Plan
	rates := plan.Request.TaxRates
	var output strings.Builder
	output.WriteString("\nSuggested Liquidation\n\n")

	sells := slices.Clone(selection.Sells)
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

	availableNow := decimal.Min(plan.Request.Amount, plan.AvailableCash)
	summaries := [][2]string{
		{"Requested withdrawal:", money(plan.Request.Amount)},
		{"Available now for this withdrawal:", money(availableNow)},
		{"Estimated sale proceeds:", money(selection.Proceeds)},
	}
	summaries = append(summaries, orderSummaries(formatted, selection.ExpectedBalance)...)
	writeSummaries(&output, summaries)

	if len(selection.AllocationWarnings) > 0 {
		output.WriteString("\nThese assets end up outside their allocation band after this sale:\n")
		for _, warning := range selection.AllocationWarnings {
			fmt.Fprintf(&output, "  %s: %s (allowed %s to %s)\n",
				warning.Asset,
				percentage(warning.Allocation),
				percentage(warning.MinAllocAllowed),
				percentage(warning.MaxAllocAllowed),
			)
		}
	}
	if formatted.washSale {
		output.WriteString("\n[WS] Possible wash sale: a recent purchase of this ticker may disallow the loss.\n")
	}
	return output.String()
}
