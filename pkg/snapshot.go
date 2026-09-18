package pkg

import (
	"context"
	"fmt"
	"maps"
	"seesaw/internal"
	"seesaw/internal/config"
	"seesaw/internal/etrade"
	"seesaw/internal/utils"
	"slices"
	"time"

	"github.com/quagmt/udecimal"
)

type Asset struct {
	Plan      *AssetPlan
	Positions []etrade.Position

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

func SnapshotPortfolio(plan *PortfolioPlan, daysBack int) (*AccountSnapshot, error) {
	ctx := context.Background()

	env, err := config.LoadEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load .env: %v", err)
	}

	client, err := etrade.NewClient(
		ctx,
		env.ETradeApiKey,
		env.ETradeApiSecret,
		env.ETradeAPIBaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ETrade: %v", err)
	}

	account, err := client.GetAccountDetails(ctx, plan.AccountId)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account key: %v", err)
	}

	portfolio, err := client.GetPortfolio(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve portfolio positions")
	}

	if daysBack <= 0 {
		daysBack = config.WashSaleWindowDays
	}

	inScope := internal.SetFrom(
		plan.Tickers(),
		maps.Keys(portfolio.PositionByTicker),
	)

	recentTx, err := client.GetRecentTransactions(ctx, account, daysBack, inScope)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account transactions")
	}

	var assets []Asset

	for _, assetPlan := range plan.Assets {
		var positions []etrade.Position
		for ticker := range assetPlan.Tickers() {
			if pos, exists := portfolio.PositionByTicker[ticker]; exists {
				positions = append(positions, pos)
				delete(portfolio.PositionByTicker, ticker)
			}
		}

		assets = append(assets, assignAsset(&assetPlan, positions, portfolio.TotalValue))
	}

	// Extra positions should be uncommon, but still technically possible.
	// Aggregate into a single Asset with a nil AssetPlan, which should
	// force Seesaw into liquidating as soon as possible.
	if len(portfolio.PositionByTicker) > 0 {
		extraPositions := slices.Collect(maps.Values(portfolio.PositionByTicker))
		assets = append(assets, assignAsset(nil, extraPositions, portfolio.TotalValue))
	}

	snapshot := AccountSnapshot{
		Id:          plan.AccountId,
		CreatedAt:   time.Now(),
		BuyingPower: portfolio.BuyingPower,
		TotalValue:  portfolio.TotalValue,
		Assets:      assets,
		RecentTx:    recentTx,
	}

	utils.DebugYAML(snapshot)

	return &snapshot, nil
}

func assignAsset(
	plan *AssetPlan,
	positions []etrade.Position,
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
