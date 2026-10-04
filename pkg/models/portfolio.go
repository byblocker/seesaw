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
	CashBalance      decimal.Decimal
	BuyingPower      decimal.Decimal
	WithdrawableCash decimal.Decimal
}

type Portfolio struct {
	Tickers          []string
	Balances         Balances
	TotalValue       decimal.Decimal
	PositionByTicker map[string]Position
	RecentTx         RecentTxByTicker
}

type Position struct {
	Id          int64
	Ticker      string
	Quantity    decimal.Decimal
	Allocation  decimal.Decimal
	MarketValue decimal.Decimal
	Lots        []PositionLot
}

type PositionLot struct {
	Id           int64
	Quantity     decimal.Decimal
	PricePaid    decimal.Decimal
	TimeAcquired time.Time
	MarketValue  decimal.Decimal
	UnrealizedPL decimal.Decimal
	PLPct        decimal.Decimal
}
