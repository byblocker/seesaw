package models

import (
	"time"

	"github.com/shopspring/decimal"
)

const WashSaleWindowDays = 30

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
