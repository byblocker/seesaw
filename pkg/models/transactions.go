package models

import "time"

type Transaction struct {
	Ticker string
	Type   TransactionType
	Date   time.Time
}

type TransactionType string

const (
	TxBuy  TransactionType = "Bought"
	TxSell TransactionType = "Sold"
)

func (t TransactionType) IsSupported() bool {
	return t == TxBuy || t == TxSell
}
