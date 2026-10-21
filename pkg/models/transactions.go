package models

import (
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

const WashSaleWindowDays = 30

type RecentTxByTicker map[string]struct {
	Buys  []Transaction
	Sells []Transaction
}

func (r RecentTxByTicker) HasBuyWithinWashWindow(ticker string, day time.Time) bool {
	return hasTradeWithinWashWindow(r[ticker].Buys, day)
}

func (r RecentTxByTicker) HasSellWithinWashWindow(ticker string, day time.Time) bool {
	return hasTradeWithinWashWindow(r[ticker].Sells, day)
}

func hasTradeWithinWashWindow(trades []Transaction, day time.Time) bool {
	windowStart := calendarDate(day).AddDate(0, 0, -WashSaleWindowDays)
	return slices.ContainsFunc(trades, func(trade Transaction) bool {
		return !calendarDate(trade.Date.In(day.Location())).Before(windowStart)
	})
}

func calendarDate(moment time.Time) time.Time {
	year, month, day := moment.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (r RecentTxByTicker) Add(transaction Transaction) {
	history := r[transaction.Ticker]

	switch transaction.Type {
	case TxBuy:
		history.Buys = append(history.Buys, transaction)
	case TxSell:
		history.Sells = append(history.Sells, transaction)
	default:
		return
	}

	r[transaction.Ticker] = history
}

func (r RecentTxByTicker) Append(other RecentTxByTicker) {
	for ticker, history := range other {
		combined := r[ticker]
		combined.Buys = append(combined.Buys, history.Buys...)
		combined.Sells = append(combined.Sells, history.Sells...)
		r[ticker] = combined
	}
}

type Transaction struct {
	Ticker   string
	Type     TransactionType
	Date     time.Time
	Quantity decimal.Decimal
	Price    decimal.Decimal
}

type TransactionType string

const (
	TxBuy  TransactionType = "Bought"
	TxSell TransactionType = "Sold"
)

func (t TransactionType) IsSupported() bool {
	return t == TxBuy || t == TxSell
}
