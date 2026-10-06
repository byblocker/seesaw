package report

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func formatSells(sells []models.SellProposal, rates *models.TaxSettings, recentTx models.RecentTxByTicker) (rows [][]string, proceeds, realizedPL decimal.Decimal, washSale bool) {
	rows = make([][]string, 0, len(sells))
	for _, sell := range sells {
		for _, lot := range sell.Lots {
			row := formatSell(sell, lot)
			if rates != nil {
				row = append(row, money(lot.EstimatedTaxImpact(*rates)))
				if lot.RealizedPL().IsNegative() && recentTx.HasRecentBuy(sell.Ticker) {
					row[1] += " [WS]"
					washSale = true
				}
			}
			rows = append(rows, row)
		}

		proceeds = proceeds.Add(sell.Proceeds)
		realizedPL = realizedPL.Add(sell.RealizedPL)
	}
	return rows, proceeds, realizedPL, washSale
}

func formatSell(sell models.SellProposal, lot models.SellLot) []string {
	proceeds := lot.Proceeds()
	realizedPL := lot.RealizedPL()

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
