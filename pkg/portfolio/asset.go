package portfolio

import (
	"seesaw/internal/collections"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

type Asset struct {
	Plan      *config.AssetPlan
	Tickers   []string
	Positions []Position

	Allocation decimal.Decimal
	AllocDrift decimal.Decimal

	Value      decimal.Decimal
	ValueDrift decimal.Decimal
}

func (asset Asset) Label() string {
	if asset.Plan == nil {
		return "OTHER"
	}
	return strings.Join(asset.Tickers, " / ")
}

func groupByAsset(
	plans []config.AssetPlan,
	portfolio *models.Portfolio,
) []Asset {
	assets := make([]Asset, 0, len(plans)+1) // includes asset for unplanned positions
	planned := collections.NewSet[string]()
	for i := range plans {
		plan := &plans[i]

		tickers := slices.Collect(plan.Tickers())
		planned.Add(tickers...)

		assets = append(assets, Asset{Plan: plan, Tickers: tickers})
	}

	var unplanned []string
	for ticker := range portfolio.PositionByTicker {
		if !planned.Contains(ticker) {
			unplanned = append(unplanned, ticker)
		}
	}

	if len(unplanned) > 0 {
		// Map iteration order is random; sort so ties between unplanned positions break the same way every run.
		slices.Sort(unplanned)
		assets = append(assets, Asset{Tickers: unplanned})
	}

	return assets
}

func (asset *Asset) allocate(totalValue decimal.Decimal) {
	if !totalValue.IsZero() {
		asset.Allocation, _ = asset.Value.QuoRem(totalValue, utils.DivisionDecimalPlaces)
	}

	if asset.Plan != nil {
		asset.AllocDrift = asset.Allocation.Sub(asset.Plan.TargetAllocation)
		expectedValue := totalValue.Mul(asset.Plan.TargetAllocation)
		asset.ValueDrift = asset.Value.Sub(expectedValue)
	}
}
