package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type Quote struct {
	Ticker string
	Ask    decimal.Decimal
	Bid    decimal.Decimal
}

type Balances struct {
	CashBalance decimal.Decimal
	BuyingPower decimal.Decimal
}

type Portfolio struct {
	Balances     Balances
	Lots         []Lot
	Transactions []Transaction
	Quotes       []Quote
}

type Lot struct {
	Ticker       string
	Id           int64
	Quantity     decimal.Decimal
	PricePaid    decimal.Decimal
	TimeAcquired time.Time
}
