package config

import (
	"github.com/shopspring/decimal"
)

type TaxSettings struct {
	ShortTermGainRate *decimal.Decimal `yaml:"short_term_gain_rate" validate:"required,gte=0,lte=1"`
	LongTermGainRate  *decimal.Decimal `yaml:"long_term_gain_rate" validate:"required,gte=0,lte=1"`
}

func (settings TaxSettings) Validate() error {
	return validateSettings(settings, "tax")
}

type RebalanceSettings struct {
	// Minimum remainder to round a whole-share purchase up.
	MinFractionalShareToRound *decimal.Decimal `yaml:"min_fractional_share_to_round" validate:"required,gte=0,lte=1"`
	// Extra purchase allowance as a fraction of total portfolio value.
	MaxPctAlocDeviation *decimal.Decimal `yaml:"max_pct_allocation_deviation" validate:"required,gte=0,lte=1"`
	// Loss threshold expressed as a nonpositive fraction of the lot's cost.
	MinPctLossToHarvest *decimal.Decimal `yaml:"min_pct_loss_to_harvest" validate:"required,gte=-1,lte=0"`
}

func (settings RebalanceSettings) Validate() error {
	return validateSettings(settings, "rebalance")
}

type LiquidationSettings struct {
	MaxAllocationDeviation *decimal.Decimal `yaml:"max_pct_allocation_deviation" validate:"required,gte=0,lte=1"`
	MaxExcessCash          *decimal.Decimal `yaml:"max_excess_cash" validate:"required,gte=0"`
}

func (settings LiquidationSettings) Validate() error {
	return validateSettings(settings, "liquidation")
}
