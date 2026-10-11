package report

import (
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"slices"
	"strings"

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
	sells = slices.SortedFunc(slices.Values(sells), func(left, right models.SellProposal) int {
		return strings.Compare(left.Ticker, right.Ticker)
	})
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

func writeAssets(output *strings.Builder, assets []portfolio.Asset) string {
	headers := []string{"Asset", "Value", "Current", "Target", "Deviation"}
	rows := make([][]string, 0, len(assets))
	for _, asset := range assets {
		rows = append(rows, formatAsset(asset))
	}
	return writeTable(output, headers, rows, 1)
}

func formatAsset(asset portfolio.Asset) []string {
	label, target, deviation := "OTHER", "—", "—"
	if asset.Plan != nil {
		label = strings.Join(slices.Collect(asset.Plan.Tickers()), " / ")
		target = percentage(asset.Plan.TargetAllocation)
		deviation = formatDeviation(asset.AllocDrift)
	}
	return []string{label, money(asset.Value), percentage(asset.Allocation), target, deviation}
}

func formatDeviation(drift decimal.Decimal) string {
	if drift.IsPositive() {
		return "+" + percentage(drift)
	}
	return percentage(drift)
}
