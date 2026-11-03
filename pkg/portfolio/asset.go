package portfolio

import (
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

type Asset struct {
	Plan         *config.AssetPlan
	Label        string
	PositionIdxs []int

	Allocation decimal.Decimal
	AllocDrift decimal.Decimal

	Value      decimal.Decimal
	ValueDrift decimal.Decimal
}

// One asset row per plan, then one for holdings outside the plan, each with a position row per ticker.
func newAssets(plans []config.AssetPlan, heldLots []models.Lot) ([]Asset, []Position, map[string]int) {
	assets := make([]Asset, 0, len(plans)+1) // includes asset for unplanned positions
	var positions []Position
	positionIdxByTicker := map[string]int{}
	addAsset := func(plan *config.AssetPlan, label string, tickers []string) {
		asset := Asset{Plan: plan, Label: label}
		for _, ticker := range tickers {
			positionIdxByTicker[ticker] = len(positions)
			asset.PositionIdxs = append(asset.PositionIdxs, len(positions))
			positions = append(positions, Position{Ticker: ticker, AssetIdx: len(assets)})
		}
		assets = append(assets, asset)
	}

	for i := range plans {
		tickers := slices.Collect(plans[i].Tickers())
		addAsset(&plans[i], strings.Join(tickers, " / "), tickers)
	}

	var unplanned []string
	for _, lot := range heldLots {
		if _, planned := positionIdxByTicker[lot.Ticker]; !planned {
			unplanned = append(unplanned, lot.Ticker)
		}
	}
	if len(unplanned) > 0 {
		// Brokers list lots in no set order; sort so ties between unplanned positions break the same way every run.
		slices.Sort(unplanned)
		addAsset(nil, "OTHER", slices.Compact(unplanned))
	}

	return assets, positions, positionIdxByTicker
}

func (asset *Asset) allocate(totalValue decimal.Decimal) {
	asset.Allocation = decimal.Zero
	if !totalValue.IsZero() {
		asset.Allocation, _ = asset.Value.QuoRem(totalValue, utils.DivisionDecimalPlaces)
	}

	if asset.Plan != nil {
		asset.AllocDrift = asset.Allocation.Sub(asset.Plan.TargetAllocation)
		expectedValue := totalValue.Mul(asset.Plan.TargetAllocation)
		asset.ValueDrift = asset.Value.Sub(expectedValue)
	}
}
