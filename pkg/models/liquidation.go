package models

import (
	"fmt"
	"seesaw/pkg/config"

	"github.com/shopspring/decimal"
)

type LiquidationRequest struct {
	Amount      decimal.Decimal
	Settings    config.LiquidationSettings
	TaxRates    *config.TaxSettings
	SearchLimit int
}

func (request LiquidationRequest) Validate() error {
	if !request.Amount.IsPositive() {
		return fmt.Errorf("withdrawal must be positive")
	}
	if request.SearchLimit <= 0 {
		return fmt.Errorf("search limit must be positive")
	}
	if err := request.Settings.Validate(); err != nil {
		return err
	}
	if request.TaxRates != nil {
		return request.TaxRates.Validate()
	}
	return nil
}

type LiquidationPlan struct {
	Request       LiquidationRequest
	StartingCash  decimal.Decimal
	AvailableCash decimal.Decimal
	MinTargetSale decimal.Decimal
	MaxTargetSale decimal.Decimal
}

// The cash left after the withdrawal, once sales raising these proceeds settle.
func (plan LiquidationPlan) BalanceAfter(proceeds decimal.Decimal) decimal.Decimal {
	return plan.StartingCash.Sub(plan.Request.Amount).Add(proceeds)
}

type LiquidationSelection struct {
	Plan               *LiquidationPlan
	Sells              []SellProposal
	Proceeds           decimal.Decimal
	ExpectedBalance    decimal.Decimal
	AllocationWarnings []AllocationWarning
}

type AllocationWarning struct {
	Asset           string
	Allocation      decimal.Decimal
	MinAllocAllowed decimal.Decimal
	MaxAllocAllowed decimal.Decimal
}
