package config

import (
	"fmt"
	"iter"
	"seesaw/internal/collections"
	"slices"
	"strings"

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
	PrimaryTicker    string          `yaml:"primary_ticker"`
	AltTicker        string          `yaml:"alternate_ticker"`
	TargetAllocation decimal.Decimal `yaml:"target_allocation"`
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

func (asset AssetPlan) Validate() error {
	if asset.PrimaryTicker == "" {
		return fmt.Errorf("missing required primary_ticker")
	}

	for ticker := range asset.Tickers() {
		if ticker != strings.ToUpper(ticker) {
			return fmt.Errorf("ticker %q must be uppercase", ticker)
		}
	}

	allocTooLarge := asset.TargetAllocation.Cmp(decimal.NewFromInt(1)) > 0
	allocNegative := asset.TargetAllocation.Cmp(decimal.Zero) <= 0

	if allocTooLarge || allocNegative {
		return fmt.Errorf(
			"invalid target allocation: %s",
			asset.TargetAllocation.StringFixed(2),
		)
	}

	return nil
}

type RebalanceSettings struct {
	// Minimum remainder to round a whole-share purchase up.
	MinFractionalShareToRound *decimal.Decimal `yaml:"min_fractional_share_to_round"`
	// Extra purchase allowance as a fraction of total portfolio value.
	MaxPctAlocDeviation *decimal.Decimal `yaml:"max_pct_allocation_deviation"`
	// Loss threshold expressed as a nonpositive fraction of the lot's cost.
	MinPctLossToHarvest *decimal.Decimal `yaml:"min_pct_loss_to_harvest"`
}

func (settings RebalanceSettings) Validate() error {
	if settings.MinFractionalShareToRound == nil {
		return fmt.Errorf("missing required rebalance.min_fractional_share_to_round")
	}
	if settings.MaxPctAlocDeviation == nil {
		return fmt.Errorf("missing required rebalance.max_pct_allocation_deviation")
	}
	if settings.MinPctLossToHarvest == nil {
		return fmt.Errorf("missing required rebalance.min_pct_loss_to_harvest")
	}

	one := decimal.NewFromInt(1)
	if settings.MinFractionalShareToRound.IsNegative() || settings.MinFractionalShareToRound.GreaterThan(one) {
		return fmt.Errorf("rebalance.min_fractional_share_to_round must be between 0 and 1")
	}
	if settings.MaxPctAlocDeviation.IsNegative() || settings.MaxPctAlocDeviation.GreaterThan(one) {
		return fmt.Errorf("rebalance.max_pct_allocation_deviation must be between 0 and 1")
	}
	if settings.MinPctLossToHarvest.LessThan(one.Neg()) || settings.MinPctLossToHarvest.IsPositive() {
		return fmt.Errorf("rebalance.min_pct_loss_to_harvest must be between -1 and 0")
	}
	return nil
}

type LiquidationSettings struct {
	MaxAllocationDeviation *decimal.Decimal `yaml:"max_pct_allocation_deviation"`
	CashReserve            *decimal.Decimal `yaml:"cash_reserve"`
	MaxExcessCash          *decimal.Decimal `yaml:"max_excess_cash"`
}

func (settings LiquidationSettings) Validate() error {
	if settings.MaxAllocationDeviation == nil {
		return fmt.Errorf("missing required liquidation.max_pct_allocation_deviation")
	}
	if settings.CashReserve == nil {
		return fmt.Errorf("missing required liquidation.cash_reserve")
	}
	if settings.MaxExcessCash == nil {
		return fmt.Errorf("missing required liquidation.max_excess_cash")
	}
	if settings.MaxAllocationDeviation.IsNegative() || settings.MaxAllocationDeviation.GreaterThan(decimal.NewFromInt(1)) {
		return fmt.Errorf("liquidation.max_pct_allocation_deviation must be between 0 and 1")
	}
	if settings.CashReserve.IsNegative() {
		return fmt.Errorf("liquidation.cash_reserve must be nonnegative")
	}
	if settings.MaxExcessCash.IsNegative() {
		return fmt.Errorf("liquidation.max_excess_cash must be nonnegative")
	}
	return nil
}
