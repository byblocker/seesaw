package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type Quote struct {
	Ask decimal.Decimal
	Bid decimal.Decimal
}

type Balances struct {
	CashBalance decimal.Decimal
	BuyingPower decimal.Decimal
}

type Portfolio struct {
	Balances         Balances
	PositionByTicker map[string]Position
	Transactions     []Transaction
	Quotes           map[string]Quote
}

type Position struct {
	Ticker string
	Lots   []PositionLot
}

type PositionLot struct {
	Id           int64
	Quantity     decimal.Decimal
	PricePaid    decimal.Decimal
	TimeAcquired time.Time
}
