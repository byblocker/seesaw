package models

import (
	"fmt"
	"seesaw/pkg/config"

	"github.com/shopspring/decimal"
)

type LiquidationRequest struct {
	Amount   decimal.Decimal
	Settings config.LiquidationSettings
	TaxRates *config.TaxSettings
}

func (request LiquidationRequest) Validate() error {
	if !request.Amount.IsPositive() {
		return fmt.Errorf("withdrawal must be positive")
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
	Request LiquidationRequest
	Sells   []SellProposal
}
