package report

import (
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func formatBuys(buys []models.Buy) (rows [][]string, cost decimal.Decimal) {
	rows = make([][]string, 0, len(buys))
	for _, buy := range buys {
		rows = append(rows, formatBuy(buy))
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
