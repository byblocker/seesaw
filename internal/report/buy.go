package report

import (
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

func formatBuys(buys []models.Buy, rates *config.TaxSettings) (rows [][]string, cost decimal.Decimal) {
	rows = make([][]string, 0, len(buys))
	buys = slices.SortedFunc(slices.Values(buys), func(left, right models.Buy) int {
		return strings.Compare(left.Ticker, right.Ticker)
	})
	for _, buy := range buys {
		row := formatBuy(buy)
		if rates != nil {
			row = append(row, "—")
		}
		rows = append(rows, row)
		cost = cost.Add(buy.EstimatedCost)
	}

	return rows, cost
}

func formatBuy(buy models.Buy) []string {
	return []string{
		buy.Ticker,
		"BUY",
		"—",
		buy.Quantity.String(),
		price(buy.Price),
		money(buy.EstimatedCost.Neg()),
		"—",
	}
}
