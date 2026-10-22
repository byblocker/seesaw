package config

import (
	"github.com/shopspring/decimal"
)

type TaxSettings struct {
	ShortTermGainRate *decimal.Decimal `yaml:"short_term_gain_rate" validate:"required,gte=0,lte=1"`
	LongTermGainRate  *decimal.Decimal `yaml:"long_term_gain_rate" validate:"required,gte=0,lte=1"`
}

func NewTaxSettings(shortTermGainRate, longTermGainRate decimal.Decimal) (TaxSettings, error) {
	settings := TaxSettings{
		ShortTermGainRate: &shortTermGainRate,
		LongTermGainRate:  &longTermGainRate,
	}
	if err := settings.Validate(); err != nil {
		return TaxSettings{}, err
	}
	return settings, nil
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

func NewRebalanceSettings(
	minFractionalShareToRound, maxPctAllocDeviation, minPctLossToHarvest decimal.Decimal,
) (RebalanceSettings, error) {
	settings := RebalanceSettings{
		MinFractionalShareToRound: &minFractionalShareToRound,
		MaxPctAlocDeviation:       &maxPctAllocDeviation,
		MinPctLossToHarvest:       &minPctLossToHarvest,
	}
	if err := settings.Validate(); err != nil {
		return RebalanceSettings{}, err
	}
	return settings, nil
}

func (settings RebalanceSettings) Validate() error {
	return validateSettings(settings, "rebalance")
}

type LiquidationSettings struct {
	MaxAllocationDeviation *decimal.Decimal `yaml:"max_pct_allocation_deviation" validate:"required,gte=0,lte=1"`
	MaxExcessCash          *decimal.Decimal `yaml:"max_excess_cash" validate:"required,gte=0"`
}

func NewLiquidationSettings(maxAllocationDeviation, maxExcessCash decimal.Decimal) (LiquidationSettings, error) {
	settings := LiquidationSettings{
		MaxAllocationDeviation: &maxAllocationDeviation,
		MaxExcessCash:          &maxExcessCash,
	}
	if err := settings.Validate(); err != nil {
		return LiquidationSettings{}, err
	}
	return settings, nil
}

func (settings LiquidationSettings) Validate() error {
	return validateSettings(settings, "liquidation")
}
