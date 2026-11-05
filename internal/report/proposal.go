package report

import (
	"seesaw/pkg/config"
	"seesaw/pkg/portfolio"
	"strings"
)

func Proposal(
	sells []portfolio.SellProposal,
	buys []portfolio.Buy,
	estimated portfolio.Snapshot,
	rates *config.TaxSettings,
) string {
	var output strings.Builder
	output.WriteString("\nSuggested Orders\n\n")

	summary := formatSells(sells, estimated, rates)
	buyRows := formatBuys(buys, estimated, rates)
	if len(summary.rows) == 0 && len(buyRows) == 0 {
		output.WriteString("No trades suggested.\n\n")
	} else {
		writeOrderTable(&output, summary, buyRows, rates)
	}

	output.WriteString("Estimated Portfolio\n\n")
	writePortfolio(&output, estimated)

	if summary.washSale {
		output.WriteString("[WS] Wash sale: a purchase of this ticker in the last 30 days disallows the loss, so it counts no tax relief.\n")
	}
	return output.String()
}
