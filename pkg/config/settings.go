package config

import (
	"seesaw/internal/validate"

	"github.com/shopspring/decimal"
)

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

// Settings as written in YAML or held in a struct, before they are checked.
// Pointers tell a missing key from an explicit zero, and the tags hold every rule.
type settingsYAML struct {
	MinFractionalShareToRound *decimal.Decimal `yaml:"min_fractional_share_to_round" validate:"required,gte=0,lte=1"`
	MinPctLossToHarvest       *decimal.Decimal `yaml:"min_pct_loss_to_harvest" validate:"required,gte=-1,lte=0"`
	AllocationUpperLimit      *decimal.Decimal `yaml:"allocation_upper_limit" validate:"required,gte=0,lte=1"`
	AllocationLowerLimit      *decimal.Decimal `yaml:"allocation_lower_limit" validate:"required,gte=0,lte=1"`
}

func (raw settingsYAML) build() (Settings, error) {
	if err := validate.Fields(raw, "settings."); err != nil {
		return Settings{}, err
	}
	return Settings{
		MinFractionalShareToRound: *raw.MinFractionalShareToRound,
		MinPctLossToHarvest:       *raw.MinPctLossToHarvest,
		AllocationUpperLimit:      *raw.AllocationUpperLimit,
		AllocationLowerLimit:      *raw.AllocationLowerLimit,
	}, nil
}
