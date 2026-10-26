package report

import (
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

	writeEstimatedPortfolio(&output, estimated, formatted)

	if formatted.washSale {
		output.WriteString("\n[WS] Wash sale: a purchase of this ticker in the last 30 days disallows the loss, so it counts no tax relief.\n")
	}
	return output.String()
}
