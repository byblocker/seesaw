package report

import (
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

type formattedSells struct {
	rows                 [][]string
	proceeds, realizedPL decimal.Decimal
	tax, relief          decimal.Decimal
	washSale, showTax    bool
}

func formatSells(sells []models.SellProposal, rates *config.TaxSettings, recentTx models.RecentTxByTicker) formattedSells {
	result := formattedSells{
		rows:    make([][]string, 0, len(sells)),
		showTax: rates != nil && len(sells) > 0,
	}
	for _, sell := range sells {
		for _, lot := range sell.Lots {
			realizedPL := lot.RealizedPL()
			row := []string{
				sell.Ticker,
				"SELL",
				lot.Lot.TimeAcquired.Format(utils.YYYY_MM_DD),
				lot.Quantity.String(),
				price(sell.Price),
				money(lot.Proceeds()),
				money(realizedPL),
			}
			if rates != nil {
				impact := lot.EstimatedTaxImpact(*rates)
				row = append(row, money(impact))
				if impact.IsNegative() {
					result.relief = result.relief.Sub(impact)
				} else {
					result.tax = result.tax.Add(impact)
				}
				if realizedPL.IsNegative() && recentTx.HasRecentBuy(sell.Ticker) {
					row[1] += " [WS]"
					result.washSale = true
				}
			}
			result.rows = append(result.rows, row)
		}

		result.proceeds = result.proceeds.Add(sell.Proceeds)
		result.realizedPL = result.realizedPL.Add(sell.RealizedPL)
	}
	return result
}
