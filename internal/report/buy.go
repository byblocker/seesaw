package report

import (
	"seesaw/pkg/orders"

	"github.com/shopspring/decimal"
)

func formatBuys(buys []orders.Buy) (rows [][]string, cost decimal.Decimal) {
	rows = make([][]string, 0, len(buys))
	for _, buy := range buys {
		rows = append(rows, formatBuy(buy))
		cost = cost.Add(buy.EstimatedCost)
	}

	return rows, cost
}

func formatBuy(buy orders.Buy) []string {
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
