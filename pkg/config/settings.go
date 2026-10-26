package config

import (
	"log"
	"seesaw/internal/utils"

	"github.com/shopspring/decimal"
)

type TaxSettings struct {
	ShortTermGainRate decimal.Decimal
	LongTermGainRate  decimal.Decimal
}

func NewTaxSettings(shortTermGainRate, longTermGainRate decimal.Decimal) (TaxSettings, error) {
	return taxYAML{
		ShortTermGainRate: &shortTermGainRate,
		LongTermGainRate:  &longTermGainRate,
	}.build()
}

func (settings TaxSettings) Validate() error {
	_, err := NewTaxSettings(settings.ShortTermGainRate, settings.LongTermGainRate)
	return err
}

// Settings as written in YAML or passed to a constructor, before they are checked.
// Pointers tell a missing key from an explicit zero, and the tags hold every rule.
type taxYAML struct {
	ShortTermGainRate *decimal.Decimal `yaml:"short_term_gain_rate" validate:"required,gte=0,lte=1"`
	LongTermGainRate  *decimal.Decimal `yaml:"long_term_gain_rate" validate:"required,gte=0,lte=1"`
}

func (raw taxYAML) build() (TaxSettings, error) {
	if err := validateSettings(raw, "tax"); err != nil {
		return TaxSettings{}, err
	}
	return TaxSettings{
		ShortTermGainRate: *raw.ShortTermGainRate,
		LongTermGainRate:  *raw.LongTermGainRate,
	}, nil
}

type RebalanceSettings struct {
	// Minimum remainder to round a whole-share purchase up.
	MinFractionalShareToRound decimal.Decimal
	// Extra purchase allowance as a fraction of total portfolio value.
	MaxPctAllocDeviation decimal.Decimal
	// Loss threshold expressed as a nonpositive fraction of the lot's cost.
	MinPctLossToHarvest decimal.Decimal
}

func NewRebalanceSettings(
	minFractionalShareToRound, maxPctAllocDeviation, minPctLossToHarvest decimal.Decimal,
) (RebalanceSettings, error) {
	return rebalanceYAML{
		MinFractionalShareToRound: &minFractionalShareToRound,
		MaxPctAllocDeviation:      &maxPctAllocDeviation,
		MinPctLossToHarvest:       &minPctLossToHarvest,
	}.build()
}

func (settings RebalanceSettings) Validate() error {
	_, err := NewRebalanceSettings(settings.MinFractionalShareToRound, settings.MaxPctAllocDeviation, settings.MinPctLossToHarvest)
	return err
}

func (settings RebalanceSettings) Log(totalValue decimal.Decimal, allowFractionalShares bool) {
	log.Print("Rebalance settings:")
	log.Printf("  - Harvest losses of %s or more", utils.FormatPercent(settings.MinPctLossToHarvest.Neg()))
	if allowFractionalShares {
		log.Print("  - Buy fractional shares")
		return
	}
	maxOvershoot := totalValue.Mul(settings.MaxPctAllocDeviation)
	log.Printf("  - Round up from %s share", settings.MinFractionalShareToRound)
	log.Printf("  - Allow at most %s deviation (~%s)", utils.FormatPercent(settings.MaxPctAllocDeviation), utils.FormatMoney(maxOvershoot))
}

type rebalanceYAML struct {
	MinFractionalShareToRound *decimal.Decimal `yaml:"min_fractional_share_to_round" validate:"required,gte=0,lte=1"`
	MaxPctAllocDeviation      *decimal.Decimal `yaml:"max_pct_allocation_deviation" validate:"required,gte=0,lte=1"`
	MinPctLossToHarvest       *decimal.Decimal `yaml:"min_pct_loss_to_harvest" validate:"required,gte=-1,lte=0"`
}

func (raw rebalanceYAML) build() (RebalanceSettings, error) {
	if err := validateSettings(raw, "rebalance"); err != nil {
		return RebalanceSettings{}, err
	}
	return RebalanceSettings{
		MinFractionalShareToRound: *raw.MinFractionalShareToRound,
		MaxPctAllocDeviation:      *raw.MaxPctAllocDeviation,
		MinPctLossToHarvest:       *raw.MinPctLossToHarvest,
	}, nil
}

type LiquidationSettings struct {
	MaxAllocationDeviation decimal.Decimal
}

func NewLiquidationSettings(maxAllocationDeviation decimal.Decimal) (LiquidationSettings, error) {
	return liquidationYAML{
		MaxAllocationDeviation: &maxAllocationDeviation,
	}.build()
}

func (settings LiquidationSettings) Validate() error {
	_, err := NewLiquidationSettings(settings.MaxAllocationDeviation)
	return err
}

type liquidationYAML struct {
	MaxAllocationDeviation *decimal.Decimal `yaml:"max_pct_allocation_deviation" validate:"required,gte=0,lte=1"`
}

func (raw liquidationYAML) build() (LiquidationSettings, error) {
	if err := validateSettings(raw, "liquidation"); err != nil {
		return LiquidationSettings{}, err
	}
	return LiquidationSettings{
		MaxAllocationDeviation: *raw.MaxAllocationDeviation,
	}, nil
}
