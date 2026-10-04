package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type AccountSnapshot struct {
	Id        string
	CreatedAt time.Time

	Balances   Balances
	TotalValue decimal.Decimal

	Tickers []string
	Assets  []Asset
}

type Asset struct {
	Plan      *AssetPlan
	Positions map[string]Position
	RecentTx  RecentTxByTicker

	Allocation decimal.Decimal
	AllocDrift decimal.Decimal

	Value      decimal.Decimal
	ValueDrift decimal.Decimal
}
