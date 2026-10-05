package models

import "github.com/shopspring/decimal"

type RecentTxByTicker map[string]struct {
	Buys  []Transaction
	Sells []Transaction
}

func (r RecentTxByTicker) HasRecentBuy(ticker string) bool {
	return ticker != "" && len(r[ticker].Buys) > 0
}

func (r RecentTxByTicker) HasRecentSell(ticker string) bool {
	return ticker != "" && len(r[ticker].Sells) > 0
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
