package report

import (
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/portfolio"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type formattedSells struct {
	rows                    [][]string
	shortTermPL, longTermPL decimal.Decimal
	tax, relief             decimal.Decimal
	washSale, showTax       bool
}

func writeOrderTable(output *strings.Builder, amountHeader string, rows [][]string, rates *config.TaxSettings) {
	headers := []string{"Ticker", "Action", "Acquired", "Shares", "Price", amountHeader, "P/L"}
	if rates != nil {
		headers = append(headers, "Tax")
	}
	writeTable(output, headers, rows, 3)
}

func formatSells(sells []portfolio.SellProposal, snapshot portfolio.Snapshot, rates *config.TaxSettings) formattedSells {
	result := formattedSells{
		rows:    make([][]string, 0, len(sells)),
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
			realizedPL := sale.Quantity.Mul(lot.GainPerShare)
			if lot.LongTerm {
				result.longTermPL = result.longTermPL.Add(realizedPL)
			} else {
				result.shortTermPL = result.shortTermPL.Add(realizedPL)
			}
			row := []string{
				ticker(sell),
				"SELL",
				lot.TimeAcquired.Format(time.DateOnly),
				sale.Quantity.String(),
				utils.FormatPrice(sell.Price),
				utils.FormatMoney(sale.Quantity.Mul(sell.Price)),
				utils.FormatMoney(realizedPL),
			}
			if rates != nil {
				impact := sale.Quantity.Mul(taxPerShare[sale.LotIdx])
				row = append(row, utils.FormatMoney(impact))
				if impact.IsNegative() {
					result.relief = result.relief.Sub(impact)
				} else {
					result.tax = result.tax.Add(impact)
				}
				if lot.IsWashSale {
					row[1] += " [WS]"
					result.washSale = true
				}
			}
			result.rows = append(result.rows, row)
		}
	}
	return result
}

func formatBuys(buys []portfolio.Buy, snapshot portfolio.Snapshot, rates *config.TaxSettings) [][]string {
	rows := make([][]string, 0, len(buys))
	ticker := func(buy portfolio.Buy) string {
		return snapshot.Positions[buy.PositionIdx].Ticker
	}
	buys = slices.SortedFunc(slices.Values(buys), func(left, right portfolio.Buy) int {
		return strings.Compare(ticker(left), ticker(right))
	})
	for _, buy := range buys {
		row := formatBuy(buy, ticker(buy))
		if rates != nil {
			row = append(row, "—")
		}
		rows = append(rows, row)
	}

	return rows
}

func formatBuy(buy portfolio.Buy, ticker string) []string {
	return []string{
		ticker,
		"BUY",
		"—",
		buy.Quantity.String(),
		utils.FormatPrice(buy.Price),
		utils.FormatMoney(buy.EstimatedCost.Neg()),
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

// Show the account after the orders, and for taxable portfolios, what the sales realize.
func writeEstimatedPortfolio(output *strings.Builder, estimated portfolio.Snapshot, sells formattedSells) {
	output.WriteString("Estimated Portfolio\n\n")
	divider := writeAssets(output, estimated.Assets)

	balances := [][2]string{
		{"Balance:", utils.FormatMoney(estimated.Balances.CashBalance)},
		{"Total:", utils.FormatMoney(estimated.TotalValue)},
	}
	if sells.showTax {
		writeSummaries(output, balances, [][2]string{
			{"Short-term P/L:", utils.FormatMoney(sells.shortTermPL)},
			{"Long-term P/L:", utils.FormatMoney(sells.longTermPL)},
			{"Estimated tax:", utils.FormatMoney(sells.tax.Sub(sells.relief))},
		})
	} else {
		writeSummaries(output, balances)
	}
	output.WriteString(divider)
	output.WriteString("\n\n")
}

func formatAsset(asset portfolio.Asset) []string {
	target, deviation := "—", "—"
	if asset.Plan != nil {
		target = utils.FormatPercent(asset.Plan.TargetAllocation)
		deviation = formatDeviation(asset.AllocDrift)
	}
	return []string{asset.Label, utils.FormatMoney(asset.Value), utils.FormatPercent(asset.Allocation), target, deviation}
}

func formatDeviation(drift decimal.Decimal) string {
	if drift.IsPositive() {
		return "+" + utils.FormatPercent(drift)
	}
	return utils.FormatPercent(drift)
}
