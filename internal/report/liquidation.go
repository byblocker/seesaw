package report

import (
	"fmt"
	"seesaw/pkg/models"
	"strings"
)

func Liquidation(selection models.LiquidationSelection) string {
	plan := selection.Plan
	rates := plan.Request.TaxRates
	var output strings.Builder
	output.WriteString("\nSuggested Liquidation\n\n")

	formatted := formatSells(selection.Sells, rates)
	if len(formatted.rows) == 0 {
		output.WriteString("No sales required.\n\n")
	} else {
		headers := []string{"Ticker", "Action", "Acquired", "Shares", "Price", "Proceeds", "P/L"}
		if rates != nil {
			headers = append(headers, "Est. tax impact")
		}
		writeTable(&output, headers, formatted.rows, 3)
	}

	cashFlow := [][2]string{
		{"Cash on hand:", money(plan.StartingCash)},
		{"Sale proceeds:", money(selection.Proceeds)},
		{"Withdrawal:", money(plan.Request.Amount.Neg())},
		summaryRule,
		{"Left over:", money(selection.ExpectedBalance)},
	}
	if formatted.showTax {
		netTax := formatted.tax.Sub(formatted.relief)
		writeSummaries(&output, cashFlow, [][2]string{{"Estimated tax:", money(netTax)}})
	} else {
		writeSummaries(&output, cashFlow)
	}

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
		output.WriteString("\n[WS] Wash sale: a purchase of this ticker in the last 30 days disallows the loss, so it counts no tax relief.\n")
	}
	return output.String()
}
