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
