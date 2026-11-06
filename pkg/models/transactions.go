package models

import "time"

type Transaction struct {
	Ticker string
	Type   TransactionType
	Date   time.Time
}

type TransactionType string

const (
	TxBuy  TransactionType = "buy"
	TxSell TransactionType = "sell"
)
