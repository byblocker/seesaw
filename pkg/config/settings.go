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

func (settings TaxSettings) Validate() error {
	_, err := taxYAML{
		ShortTermGainRate: &settings.ShortTermGainRate,
		LongTermGainRate:  &settings.LongTermGainRate,
	}.build()
	return err
}

// Settings as written in YAML or held in a struct, before they are checked.
// Pointers tell a missing key from an explicit zero, and the tags hold every rule.
type taxYAML struct {
	ShortTermGainRate *decimal.Decimal `yaml:"short_term_gain_rate" validate:"required,gte=0,lte=1"`
	LongTermGainRate  *decimal.Decimal `yaml:"long_term_gain_rate" validate:"required,gte=0,lte=1"`
}

func (raw taxYAML) build() (TaxSettings, error) {
	if err := utils.ValidateFields(raw, "tax."); err != nil {
		return TaxSettings{}, err
	}
	return TaxSettings{
		ShortTermGainRate: *raw.ShortTermGainRate,
		LongTermGainRate:  *raw.LongTermGainRate,
	}, nil
}

type Settings struct {
	// Minimum remainder to round a whole-share purchase up.
	MinFractionalShareToRound decimal.Decimal
	// Loss threshold expressed as a nonpositive fraction of the lot's cost.
	MinPctLossToHarvest decimal.Decimal
	// How far rounding a buy up may lift an asset above its target, as a fraction of total portfolio value.
	AllocationUpperLimit decimal.Decimal
	// How far a sale may drop an asset below its target, as a fraction of the portfolio after the withdrawal.
	AllocationLowerLimit decimal.Decimal
}

func (settings Settings) Validate() error {
	_, err := settingsYAML{
		MinFractionalShareToRound: &settings.MinFractionalShareToRound,
		MinPctLossToHarvest:       &settings.MinPctLossToHarvest,
		AllocationUpperLimit:      &settings.AllocationUpperLimit,
		AllocationLowerLimit:      &settings.AllocationLowerLimit,
	}.build()
	return err
}

func (settings Settings) Log(totalValue decimal.Decimal, allowFractionalShares bool) {
	log.Print("Rebalance settings:")
	log.Printf("  - Harvest losses of %s or more", utils.FormatPercent(settings.MinPctLossToHarvest.Neg()))
	if allowFractionalShares {
		log.Print("  - Buy fractional shares")
		return
	}
	maxOvershoot := totalValue.Mul(settings.AllocationUpperLimit)
	log.Printf("  - Round up from %s share", settings.MinFractionalShareToRound)
	log.Printf("  - Allow at most %s above target (~%s)", utils.FormatPercent(settings.AllocationUpperLimit), utils.FormatMoney(maxOvershoot))
}

type settingsYAML struct {
	MinFractionalShareToRound *decimal.Decimal `yaml:"min_fractional_share_to_round" validate:"required,gte=0,lte=1"`
	MinPctLossToHarvest       *decimal.Decimal `yaml:"min_pct_loss_to_harvest" validate:"required,gte=-1,lte=0"`
	AllocationUpperLimit      *decimal.Decimal `yaml:"allocation_upper_limit" validate:"required,gte=0,lte=1"`
	AllocationLowerLimit      *decimal.Decimal `yaml:"allocation_lower_limit" validate:"required,gte=0,lte=1"`
}

func (raw settingsYAML) build() (Settings, error) {
	if err := utils.ValidateFields(raw, "settings."); err != nil {
		return Settings{}, err
	}
	return Settings{
		MinFractionalShareToRound: *raw.MinFractionalShareToRound,
		MinPctLossToHarvest:       *raw.MinPctLossToHarvest,
		AllocationUpperLimit:      *raw.AllocationUpperLimit,
		AllocationLowerLimit:      *raw.AllocationLowerLimit,
	}, nil
}
