package report

import (
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/portfolio"
	"slices"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/shopspring/decimal"
)

type sellSummary struct {
	rows                    []table.Row
	shortTermPL, longTermPL decimal.Decimal
	tax                     decimal.Decimal
	washSale, showTax       bool
}

func writeOrderTable(output *strings.Builder, rows []table.Row, rates *config.TaxSettings) {
	headers := table.Row{"Ticker", "Action", "Acquired", "Shares", "Price", "Cost", "P/L"}
	if rates != nil {
		headers = append(headers, "Tax")
	}
	orders := newTable(headers, 3)
	orders.AppendRows(rows)
	writeTable(output, orders)
}

func formatSells(sells []portfolio.SellProposal, snapshot portfolio.Snapshot, rates *config.TaxSettings) sellSummary {
	summary := sellSummary{
		rows:    make([]table.Row, 0, len(sells)),
		showTax: rates != nil && len(sells) > 0,
	}
	taxPerShare := portfolio.TaxPerShare(snapshot.Lots, rates)
	ticker := func(sell portfolio.SellProposal) string {
		return snapshot.Positions[sell.PositionIdx].Ticker
	}
	sells = slices.SortedFunc(slices.Values(sells), func(left, right portfolio.SellProposal) int {
		return strings.Compare(ticker(left), ticker(right))
	})
	for _, sell := range sells {
		for _, sale := range sell.Lots {
			lot := snapshot.Lots[sale.LotIdx]
			realizedPL := sale.Quantity.Mul(lot.Snapshot.GainPerShare)
			if lot.Snapshot.LongTerm {
				summary.longTermPL = summary.longTermPL.Add(realizedPL)
			} else {
				summary.shortTermPL = summary.shortTermPL.Add(realizedPL)
			}
			action := "SELL"
			if rates != nil && lot.Snapshot.IsWashSale {
				action += " [WS]"
				summary.washSale = true
			}
			row := table.Row{
				ticker(sell),
				action,
				lot.TimeAcquired.Format(time.DateOnly),
				sale.Quantity.String(),
				utils.FormatPrice(lot.Snapshot.Bid),
				utils.FormatMoney(sale.Quantity.Mul(lot.Snapshot.Bid)),
				utils.FormatMoney(realizedPL),
			}
			if rates != nil {
				saleTax := sale.Quantity.Mul(taxPerShare[sale.LotIdx])
				row = append(row, utils.FormatMoney(saleTax))
				summary.tax = summary.tax.Add(saleTax)
			}
			summary.rows = append(summary.rows, row)
		}
	}
	return summary
}

func formatBuys(buys []portfolio.Buy, snapshot portfolio.Snapshot, rates *config.TaxSettings) []table.Row {
	rows := make([]table.Row, 0, len(buys))
	ticker := func(buy portfolio.Buy) string {
		return snapshot.Positions[buy.PositionIdx].Ticker
	}
	buys = slices.SortedFunc(slices.Values(buys), func(left, right portfolio.Buy) int {
		return strings.Compare(ticker(left), ticker(right))
	})
	for _, buy := range buys {
		row := table.Row{
			ticker(buy),
			"BUY",
			"—",
			buy.Quantity.String(),
			utils.FormatPrice(buy.Price),
			utils.FormatMoney(buy.EstimatedCost.Neg()),
			"—",
		}
		if rates != nil {
			row = append(row, "—")
		}
		rows = append(rows, row)
	}

	return rows
}

func writeAssets(output *strings.Builder, assets []portfolio.Asset) {
	assetTable := newTable(table.Row{"Asset", "Value", "Current", "Target", "Deviation"}, 1)
	for _, asset := range assets {
		assetTable.AppendRow(formatAsset(asset))
	}
	writeTable(output, assetTable)
}

// Show the account after the orders, and for taxable portfolios, what the sales realize.
func writeEstimatedPortfolio(output *strings.Builder, estimated portfolio.Snapshot, summary sellSummary) {
	output.WriteString("Estimated Portfolio\n\n")
	if summary.showTax {
		writePortfolio(output, estimated, [][2]string{
			{"Short-term P/L:", utils.FormatMoney(summary.shortTermPL)},
			{"Long-term P/L:", utils.FormatMoney(summary.longTermPL)},
			{"Estimated tax:", utils.FormatMoney(summary.tax)},
		})
	} else {
		writePortfolio(output, estimated)
	}
}

func formatAsset(asset portfolio.Asset) table.Row {
	target, deviation := "—", "—"
	if asset.Plan != nil {
		target = utils.FormatPercent(asset.Plan.TargetAllocation)
		deviation = formatDeviation(asset.AllocDrift)
	}
	return table.Row{asset.Label, utils.FormatMoney(asset.Value), utils.FormatPercent(asset.Allocation), target, deviation}
}

func formatDeviation(drift decimal.Decimal) string {
	if drift.IsPositive() {
		return "+" + utils.FormatPercent(drift)
	}
	return utils.FormatPercent(drift)
}
