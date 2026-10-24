package report

import (
	"fmt"
	"seesaw/internal/utils"
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
		writeOrderTable(&output, "Proceeds", formatted.rows, rates)
	}

	cashFlow := [][2]string{
		{"Cash on hand:", utils.FormatMoney(plan.StartingCash)},
		{"Sale proceeds:", utils.FormatMoney(selection.Proceeds)},
		{"Withdrawal:", utils.FormatMoney(plan.Request.Amount.Neg())},
		summaryRule,
		{"Left over:", utils.FormatMoney(selection.ExpectedBalance())},
	}
	if formatted.showTax {
		netTax := formatted.tax.Sub(formatted.relief)
		writeSummaries(&output, cashFlow, [][2]string{{"Estimated tax:", utils.FormatMoney(netTax)}})
	} else {
		writeSummaries(&output, cashFlow)
	}

	if len(selection.AllocationWarnings) > 0 {
		output.WriteString("\nThese assets end up outside their allocation band after this sale:\n")
		for _, warning := range selection.AllocationWarnings {
			fmt.Fprintf(&output, "  %s: %s (allowed %s to %s)\n",
				warning.Asset,
				utils.FormatPercent(warning.Allocation),
				utils.FormatPercent(warning.MinAllocAllowed),
				utils.FormatPercent(warning.MaxAllocAllowed),
			)
		}
	}
	if formatted.washSale {
		output.WriteString("\n[WS] Wash sale: a purchase of this ticker in the last 30 days disallows the loss, so it counts no tax relief.\n")
	}
	return output.String()
}
