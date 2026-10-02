package report

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func formatSells(sells []models.SellProposal) (rows [][]string, proceeds, realizedPL decimal.Decimal) {
	rows = make([][]string, 0, len(sells))
	for _, sell := range sells {
		for _, lot := range sell.Lots {
			rows = append(rows, formatSell(sell, lot))
		}

		proceeds = proceeds.Add(sell.Proceeds)
		realizedPL = realizedPL.Add(sell.RealizedPL)
	}
	return rows, proceeds, realizedPL
}

func formatSell(sell models.SellProposal, lot models.SellLot) []string {
	proceeds := lot.Quantity.Mul(sell.Price)
	realizedPL := lot.Quantity.Mul(sell.Price.Sub(lot.Lot.PricePaid))

	return []string{
		sell.Ticker,
		"SELL",
		lot.Lot.TimeAcquired.Format(utils.YYYY_MM_DD),
		lot.Quantity.String(),
		price(sell.Price),
		money(proceeds),
		money(realizedPL),
	}
}
