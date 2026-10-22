package config

import (
	"iter"
	"seesaw/internal/collections"
	"slices"

	"github.com/shopspring/decimal"
)

type Plan struct {
	Broker      string              `yaml:"broker"`
	AccountId   string              `yaml:"accountId"`
	IsTaxable   *bool               `yaml:"is_taxable"`
	Rebalance   RebalanceSettings   `yaml:"rebalance"`
	Liquidation LiquidationSettings `yaml:"liquidation"`
	Assets      []AssetPlan         `yaml:"assets"`
}

func (plan *Plan) Tickers() iter.Seq[string] {
	return collections.FlatMap(slices.Values(plan.Assets), AssetPlan.Tickers)
}

type AssetPlan struct {
	PrimaryTicker    string          `yaml:"primary_ticker" validate:"required,uppercase"`
	AltTicker        string          `yaml:"alternate_ticker" validate:"omitempty,uppercase"`
	TargetAllocation decimal.Decimal `yaml:"target_allocation" validate:"gt=0,lte=1"`
}

func (asset AssetPlan) Tickers() iter.Seq[string] {
	return func(yield func(string) bool) {
		yieldTicker := func(ticker string) bool {
			return ticker == "" || yield(ticker)
		}
		if !yieldTicker(asset.PrimaryTicker) || !yieldTicker(asset.AltTicker) {
			return
		}
	}
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
