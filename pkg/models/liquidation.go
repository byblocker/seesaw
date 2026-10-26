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

type LiquidationSelection struct {
	Request      LiquidationRequest
	StartingCash decimal.Decimal
	Sells        []SellProposal
	Proceeds     decimal.Decimal
}

func (selection LiquidationSelection) ExpectedBalance() decimal.Decimal {
	return selection.StartingCash.Sub(selection.Request.Amount).Add(selection.Proceeds)
}
