package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type AccountSnapshot struct {
	Id        string
	CreatedAt time.Time

	Portfolio *Portfolio
	Assets    []Asset
}

type Asset struct {
	Plan    *AssetPlan
	Tickers []string

	Allocation decimal.Decimal
	AllocDrift decimal.Decimal

	Value      decimal.Decimal
	ValueDrift decimal.Decimal
}
