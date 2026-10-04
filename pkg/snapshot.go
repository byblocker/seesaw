package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/collections"
	"seesaw/internal/rebalance"
	"seesaw/pkg/models"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

// Allocation ratios retain 19 decimal places to limit intermediate precision loss.
const allocationDecimalPlaces int32 = 19

func SnapshotPortfolio(
	ctx context.Context,
	client models.BrokerClient,
	plan *models.PortfolioPlan,
) (*models.AccountSnapshot, error) {
	portfolio, err := client.Portfolio(
		ctx,
		plan.AccountId,
		rebalance.WashSaleWindowDays,
		slices.Collect(plan.Tickers()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve portfolio: %w", err)
	}

	recentTx := portfolio.RecentTx

	var assets []models.Asset

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
	// Aggregate into a single unplanned asset for tax-loss harvesting.
	if len(portfolio.PositionByTicker) > 0 {
		unplanned := assignAsset(
			nil,
			portfolio.PositionByTicker,
			recentTx,
			portfolio.TotalValue,
		)
		assets = append(assets, unplanned)
	}

	return &models.AccountSnapshot{
		Id:         plan.AccountId,
		CreatedAt:  time.Now(),
		Balances:   portfolio.Balances,
		TotalValue: portfolio.TotalValue,
		Tickers:    portfolio.Tickers,
		Assets:     assets,
	}, nil
}

func assignAsset(
	plan *models.AssetPlan,
	positions map[string]models.Position,
	recentTx models.RecentTxByTicker,
	portfolioValue decimal.Decimal,
) models.Asset {
	asset := models.Asset{Plan: plan, Positions: positions, RecentTx: recentTx}
	for _, pos := range positions {
		asset.Value = asset.Value.Add(pos.MarketValue)
	}

	if !portfolioValue.IsZero() {
		asset.Allocation, _ = asset.Value.QuoRem(portfolioValue, allocationDecimalPlaces)
	}

	if plan != nil {
		asset.AllocDrift = asset.Allocation.Sub(plan.TargetAllocation)
		expectedValue := portfolioValue.Mul(plan.TargetAllocation)
		asset.ValueDrift = asset.Value.Sub(expectedValue)
	}

	return asset
}
