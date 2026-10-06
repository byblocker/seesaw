package portfolio

import (
	"seesaw/internal/collections"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"slices"

	"github.com/shopspring/decimal"
)

type Asset struct {
	Plan    *config.AssetPlan
	Tickers []string

	Allocation decimal.Decimal
	AllocDrift decimal.Decimal

	Value      decimal.Decimal
	ValueDrift decimal.Decimal
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

		assets = append(assets, assignAsset(plan, tickers, portfolio))
	}

	var unplanned []string
	for ticker := range portfolio.PositionByTicker {
		if !planned.Contains(ticker) {
			unplanned = append(unplanned, ticker)
		}
	}

	if len(unplanned) > 0 {
		assets = append(assets, assignAsset(nil, unplanned, portfolio))
	}

	return assets
}

func assignAsset(
	plan *config.AssetPlan,
	tickers []string,
	portfolio *models.Portfolio,
) Asset {
	asset := Asset{Plan: plan, Tickers: tickers}
	for _, ticker := range tickers {
		asset.Value = asset.Value.Add(portfolio.PositionByTicker[ticker].MarketValue)
	}

	if !portfolio.TotalValue.IsZero() {
		asset.Allocation, _ = asset.Value.QuoRem(portfolio.TotalValue, utils.DivisionDecimalPlaces)
	}

	if plan != nil {
		asset.AllocDrift = asset.Allocation.Sub(plan.TargetAllocation)
		expectedValue := portfolio.TotalValue.Mul(plan.TargetAllocation)
		asset.ValueDrift = asset.Value.Sub(expectedValue)
	}

	return asset
}
