package pkg

import (
	"context"
	"fmt"
	"maps"
	"seesaw/internal/collections"
	"seesaw/internal/config"
	"seesaw/pkg/etrade"
	"time"

	"github.com/quagmt/udecimal"
)

type Asset struct {
	Plan      *AssetPlan
	Positions map[string]etrade.Position

	Allocation udecimal.Decimal
	AllocDrift udecimal.Decimal

	Value      udecimal.Decimal
	ValueDrift udecimal.Decimal
}

type AccountSnapshot struct {
	Id          string
	CreatedAt   time.Time
	BuyingPower udecimal.Decimal
	TotalValue  udecimal.Decimal
	Assets      []Asset
	RecentTx    etrade.RecentTxByTicker
}

func SnapshotPortfolio(
	ctx context.Context,
	client *etrade.Client,
	plan *PortfolioPlan,
	daysBack int,
) (*AccountSnapshot, error) {
	portfolio, err := client.GetPortfolio(ctx, plan.AccountId)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve portfolio positions: %w", err)
	}

	if daysBack <= 0 {
		daysBack = config.WashSaleWindowDays
	}

	inScope := collections.SetFrom(
		plan.Tickers(),
		maps.Keys(portfolio.PositionByTicker),
	)

	recentTx, err := client.GetRecentTransactions(
		ctx,
		plan.AccountId,
		daysBack,
		inScope.Slice(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account transactions: %w", err)
	}

	var assets []Asset

	for _, assetPlan := range plan.Assets {
		positions := collections.ExtractMapByKeys(
			portfolio.PositionByTicker,
			assetPlan.Tickers(),
		)
		planned := assignAsset(&assetPlan, positions, portfolio.TotalValue)

		assets = append(assets, planned)
	}

	// Extra positions should be uncommon, but still technically possible.
	// Aggregate into a single Asset with a nil AssetPlan, which should
	// force Seesaw into liquidating as soon as possible.
	if len(portfolio.PositionByTicker) > 0 {
		unplanned := assignAsset(nil, portfolio.PositionByTicker, portfolio.TotalValue)
		assets = append(assets, unplanned)
	}

	return &AccountSnapshot{
		Id:          plan.AccountId,
		CreatedAt:   time.Now(),
		BuyingPower: portfolio.BuyingPower,
		TotalValue:  portfolio.TotalValue,
		Assets:      assets,
		RecentTx:    recentTx,
	}, nil
}

func assignAsset(
	plan *AssetPlan,
	positions map[string]etrade.Position,
	portfolioValue udecimal.Decimal,
) Asset {
	asset := Asset{Plan: plan, Positions: positions}
	for _, pos := range positions {
		asset.Value = asset.Value.Add(pos.MarketValue)
	}

	if !portfolioValue.IsZero() {
		asset.Allocation, _ = asset.Value.Div(portfolioValue)
	}

	if plan != nil {
		asset.AllocDrift = asset.Allocation.Sub(plan.TargetAllocation)
		expectedValue := portfolioValue.Mul(plan.TargetAllocation)
		asset.ValueDrift = asset.Value.Sub(expectedValue)
	}

	return asset
}
