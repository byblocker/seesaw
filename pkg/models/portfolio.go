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
	Balances         Balances
	PositionByTicker map[string]Position
	RecentTx         RecentTxByTicker
}

type Position struct {
	Id       int64
	Ticker   string
	Quantity decimal.Decimal
	Lots     []PositionLot
}

type PositionLot struct {
	Id           int64
	Quantity     decimal.Decimal
	PricePaid    decimal.Decimal
	TimeAcquired time.Time
}

func (lot PositionLot) IsLongTermAt(saleDate time.Time) bool {
	acquiredYear, acquiredMonth, acquiredDay := lot.TimeAcquired.In(saleDate.Location()).Date()
	soldYear, soldMonth, soldDay := saleDate.Date()

	// The holding period starts the day after acquisition; the sale day counts.
	// UTC date-only values keep time of day and DST out of the calculation.
	holdingStart := time.Date(acquiredYear, acquiredMonth, acquiredDay+1, 0, 0, 0, 0, time.UTC)

	// Shift the inclusive sale date back one year instead of advancing the start.
	// This preserves the boundary when either year includes February 29.
	saleOneYearEarlier := time.Date(soldYear-1, soldMonth, soldDay, 0, 0, 0, 0, time.UTC)
	return !saleOneYearEarlier.Before(holdingStart)
}
