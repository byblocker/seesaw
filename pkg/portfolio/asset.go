package portfolio

import (
	"seesaw/internal/collections"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"slices"
	"strings"
	"time"

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

// One position row per ticker, grouped by asset in plan order, with unplanned positions last.
func groupByAsset(
	plans []config.AssetPlan,
	portfolio *models.Portfolio,
	createdAt time.Time,
) ([]Asset, []Position, []LotSnapshot, error) {
	assets := make([]Asset, 0, len(plans)+1) // includes asset for unplanned positions

	quoteByTicker := make(map[string]models.Quote, len(portfolio.Quotes))
	for _, quote := range portfolio.Quotes {
		quoteByTicker[quote.Ticker] = quote
	}

	lotsByTicker := map[string][]models.PositionLot{}
	for _, lot := range portfolio.Lots {
		lotsByTicker[lot.Ticker] = append(lotsByTicker[lot.Ticker], lot)
	}

	var positions []Position
	var lots []LotSnapshot
	addAsset := func(plan *config.AssetPlan, label string, tickers []string) error {
		asset := Asset{Plan: plan, Label: label}
		for _, ticker := range tickers {
			asset.PositionIdxs = append(asset.PositionIdxs, len(positions))
			position, held, err := newPosition(ticker, len(assets), quoteByTicker[ticker], lotsByTicker[ticker], portfolio, createdAt)
			if err != nil {
				return err
			}
			for _, lot := range held {
				lot.PositionIdx = len(positions)
				position.LotIdxs = append(position.LotIdxs, len(lots))
				lots = append(lots, lot)
			}
			positions = append(positions, position)
		}
		assets = append(assets, asset)
		return nil
	}

	planned := collections.NewSet[string]()
	for i := range plans {
		plan := &plans[i]

		tickers := slices.Collect(plan.Tickers())
		planned.Add(tickers...)

		if err := addAsset(plan, strings.Join(tickers, " / "), tickers); err != nil {
			return nil, nil, nil, err
		}
	}

	var unplanned []string
	for ticker := range lotsByTicker {
		if !planned.Contains(ticker) {
			unplanned = append(unplanned, ticker)
		}
	}

	if len(unplanned) > 0 {
		// Map iteration order is random; sort so ties between unplanned positions break the same way every run.
		slices.Sort(unplanned)
		if err := addAsset(nil, "OTHER", unplanned); err != nil {
			return nil, nil, nil, err
		}
	}

	return assets, positions, lots, nil
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
