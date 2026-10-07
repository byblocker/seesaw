package models

import (
	"seesaw/pkg/config"

	"github.com/shopspring/decimal"
)

type LiquidationRequest struct {
	Amount      decimal.Decimal
	Settings    config.LiquidationSettings
	TaxRates    *config.TaxSettings
	SearchLimit int
}

type LiquidationPlan struct {
	Request       LiquidationRequest
	StartingCash  decimal.Decimal
	AvailableCash decimal.Decimal
	MinTargetSale decimal.Decimal
	MaxTargetSale decimal.Decimal
}

type LiquidationSelection struct {
	Plan            *LiquidationPlan
	Sells           []SellProposal
	Proceeds        decimal.Decimal
	ExpectedBalance decimal.Decimal
}
