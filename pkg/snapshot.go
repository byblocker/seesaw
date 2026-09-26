package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/collections"
	"seesaw/internal/config"
	"seesaw/pkg/models"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

type AccountSnapshot struct {
	Id        string
	CreatedAt time.Time

	BuyingPower decimal.Decimal
	TotalValue  decimal.Decimal

	Tickers []string
	Assets  []Asset
}

type Asset struct {
	Plan      *AssetPlan
	Positions map[string]models.Position
	RecentTx  models.RecentTxByTicker

	Allocation decimal.Decimal
	AllocDrift decimal.Decimal

	Value      decimal.Decimal
	ValueDrift decimal.Decimal
}

func SnapshotPortfolio(
	ctx context.Context,
	client models.Broker,
	plan *PortfolioPlan,
	daysBack int,
) (*AccountSnapshot, error) {
	if daysBack <= 0 {
		daysBack = config.WashSaleWindowDays
	}
	portfolio, err := client.Portfolio(ctx, plan.AccountId, daysBack, slices.Collect(plan.Tickers()))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve portfolio: %w", err)
	}

	recentTx := portfolio.RecentTx

	var assets []Asset

	for _, assetPlan := range plan.Assets {
		positions := collections.ExtractMapByKeys(
			portfolio.PositionByTicker,
			assetPlan.Tickers(),
		)
		transactions := collections.ExtractMapByKeys(
			recentTx,
			assetPlan.Tickers(),
		)
		planned := assignAsset(
			&assetPlan,
			positions,
			transactions,
			portfolio.TotalValue,
		)

		assets = append(assets, planned)
	}

	// Extra positions should be uncommon, but still technically possible.
	// Aggregate into a single Asset with a nil AssetPlan, which should
	// force Seesaw into liquidating as soon as possible.
	if len(portfolio.PositionByTicker) > 0 {
		unplanned := assignAsset(
			nil,
			portfolio.PositionByTicker,
			recentTx,
			portfolio.TotalValue,
		)
		assets = append(assets, unplanned)
	}

	return &AccountSnapshot{
		Id:          plan.AccountId,
		CreatedAt:   time.Now(),
		BuyingPower: portfolio.BuyingPower,
		TotalValue:  portfolio.TotalValue,
		Tickers:     portfolio.Tickers,
		Assets:      assets,
	}, nil
}

func assignAsset(
	plan *AssetPlan,
	positions map[string]models.Position,
	recentTx models.RecentTxByTicker,
	portfolioValue decimal.Decimal,
) Asset {
	asset := Asset{Plan: plan, Positions: positions, RecentTx: recentTx}
	for _, pos := range positions {
		asset.Value = asset.Value.Add(pos.MarketValue)
	}

	if !portfolioValue.IsZero() {
		asset.Allocation, _ = asset.Value.QuoRem(portfolioValue, config.DivisionDecimalPlaces)
	}

	if plan != nil {
		asset.AllocDrift = asset.Allocation.Sub(plan.TargetAllocation)
		expectedValue := portfolioValue.Mul(plan.TargetAllocation)
		asset.ValueDrift = asset.Value.Sub(expectedValue)
	}

	return asset
}
