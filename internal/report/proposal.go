package report

import (
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"strings"
)

func Proposal(
	sells []models.SellProposal,
	buys []models.Buy,
	estimated portfolio.Snapshot,
	rates *config.TaxSettings,
) string {
	var output strings.Builder
	output.WriteString("\nSuggested Orders\n\n")

	formatted := formatSells(sells, rates)
	rows := append(formatted.rows, formatBuys(buys, rates)...)
	if len(rows) == 0 {
		output.WriteString("No trades suggested.\n\n")
	} else {
		writeOrderTable(&output, "Cost", rows, rates)
	}

	writeEstimatedPortfolio(&output, estimated, formatted)

	if formatted.washSale {
		output.WriteString("\n[WS] Wash sale: a purchase of this ticker in the last 30 days disallows the loss, so it counts no tax relief.\n")
	}
	return output.String()
}
