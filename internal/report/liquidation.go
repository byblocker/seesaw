package report

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"strings"
)

func Liquidation(selection models.LiquidationSelection, estimated portfolio.Snapshot) string {
	rates := selection.Request.TaxRates
	var output strings.Builder
	output.WriteString("\nSuggested Orders\n\n")

	formatted := formatSells(selection.Sells, rates)
	if len(formatted.rows) == 0 {
		output.WriteString("No sales required.\n\n")
	} else {
		writeOrderTable(&output, "Proceeds", formatted.rows, rates)
	}

	cashFlow := [][2]string{
		{"Cash on hand:", utils.FormatMoney(selection.StartingCash)},
		{"Sale proceeds:", utils.FormatMoney(selection.Proceeds)},
		{"Withdrawal:", utils.FormatMoney(selection.Request.Amount.Neg())},
		summaryRule,
		{"Left over:", utils.FormatMoney(selection.ExpectedBalance())},
	}
	if formatted.showTax {
		netTax := formatted.tax.Sub(formatted.relief)
		writeSummaries(&output, cashFlow, [][2]string{{"Estimated tax:", utils.FormatMoney(netTax)}})
	} else {
		writeSummaries(&output, cashFlow)
	}

	writeEstimatedPortfolio(&output, estimated)

	if formatted.washSale {
		output.WriteString("\n[WS] Wash sale: a purchase of this ticker in the last 30 days disallows the loss, so it counts no tax relief.\n")
	}
	return output.String()
}
