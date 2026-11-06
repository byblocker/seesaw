package report

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"
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
	washSale                bool
}

// Write the orders, and for taxable portfolios with sales, what the sales realize.
func writeOrderTable(output *strings.Builder, sales sellSummary, buys []table.Row, taxable bool) {
	headers := table.Row{"Ticker", "Action", "Acquired", "Shares", "Price", "Cost", "P/L"}
	if taxable {
		headers = append(headers, "Tax")
	}
	orders := newTable(headers, 3)
	orders.AppendRows(sales.rows)
	orders.AppendRows(buys)
	if taxable && len(sales.rows) > 0 {
		orders.AppendFooter(table.Row{"", "", "", "", "Short-term", "", utils.FormatMoney(sales.shortTermPL)})
		orders.AppendFooter(table.Row{"", "", "", "", "Long-term", "", utils.FormatMoney(sales.longTermPL)})
		orders.AppendFooter(table.Row{"", "", "", "", "Total", "",
			utils.FormatMoney(sales.shortTermPL.Add(sales.longTermPL)), utils.FormatMoney(sales.tax)})
	}
	writeTable(output, orders)
}

func formatSells(sells []portfolio.SellProposal, snapshot portfolio.Snapshot) sellSummary {
	taxable := snapshot.TaxRates != nil
	summary := sellSummary{rows: make([]table.Row, 0, len(sells))}
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
			action := string(models.TxSell)
			if taxable && lot.Snapshot.IsWashSale {
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
			if taxable {
				saleTax := sale.Quantity.Mul(lot.Snapshot.TaxPerShare)
				row = append(row, utils.FormatMoney(saleTax))
				summary.tax = summary.tax.Add(saleTax)
			}
			summary.rows = append(summary.rows, row)
		}
	}
	return summary
}

// Buy rows end at the cost, leaving the acquisition date, P/L, and tax blank.
func formatBuys(buys []portfolio.Buy, snapshot portfolio.Snapshot) []table.Row {
	rows := make([]table.Row, 0, len(buys))
	ticker := func(buy portfolio.Buy) string {
		return snapshot.Positions[buy.PositionIdx].Ticker
	}
	buys = slices.SortedFunc(slices.Values(buys), func(left, right portfolio.Buy) int {
		return strings.Compare(ticker(left), ticker(right))
	})
	for _, buy := range buys {
		rows = append(rows, table.Row{
			ticker(buy),
			string(models.TxBuy),
			"",
			buy.Quantity.String(),
			utils.FormatPrice(buy.Price),
			utils.FormatMoney(buy.EstimatedCost.Neg()),
		})
	}

	return rows
}

func writePortfolio(output *strings.Builder, snapshot portfolio.Snapshot) {
	assets := newTable(table.Row{"Asset", "Value", "Current", "Target", "Deviation"}, 1)
	for _, asset := range snapshot.Assets {
		assets.AppendRow(formatAsset(asset))
	}
	assets.AppendRow(table.Row{"Cash", utils.FormatMoney(snapshot.Balances.CashBalance)})
	assets.AppendFooter(table.Row{"Total", utils.FormatMoney(snapshot.TotalValue)})
	writeTable(output, assets)
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
