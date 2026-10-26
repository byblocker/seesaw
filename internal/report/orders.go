package report

import (
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"strings"
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

	writeEstimatedPortfolio(&output, estimated, sells)
	return output.String()
}
