package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/collections"
	"seesaw/internal/rebalance"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"slices"
	"time"
)

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

	assets := make([]models.Asset, 0, len(plan.Assets)+1) // includes asset for unplanned positions
	planned := collections.NewSet[string]()
	for i := range plan.Assets {
		assetPlan := &plan.Assets[i]

		tickers := slices.Collect(assetPlan.Tickers())
		planned.Add(tickers...)

		assets = append(assets, assignAsset(assetPlan, tickers, portfolio))
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

	return &models.AccountSnapshot{
		Id:        plan.AccountId,
		CreatedAt: time.Now(),
		Portfolio: portfolio,
		Assets:    assets,
	}, nil
}

func assignAsset(
	plan *models.AssetPlan,
	tickers []string,
	portfolio *models.Portfolio,
) models.Asset {
	asset := models.Asset{Plan: plan, Tickers: tickers}
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
