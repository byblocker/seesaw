package models

import "github.com/shopspring/decimal"

type RebalancePlan struct {
	Buys             []Buy
	Sells            []SellProposal
	RemainingBalance decimal.Decimal
}
