package pkg

import (
	"context"
	"fmt"
	"maps"
	"seesaw/internal/collections"
	"seesaw/internal/config"
	"seesaw/pkg/etrade"
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
	Positions map[string]etrade.Position
	RecentTx  etrade.RecentTxByTicker

	Allocation decimal.Decimal
	AllocDrift decimal.Decimal

	Value      decimal.Decimal
	ValueDrift decimal.Decimal
}

func SnapshotPortfolio(
	ctx context.Context,
	client *etrade.Client,
	plan *PortfolioPlan,
	daysBack int,
) (*AccountSnapshot, error) {
	accounts, err := client.GetAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieve active brokerage accounts: %w", err)
	}

	var account *etrade.AccountDetails

	for i := range accounts {
		if accounts[i].Id == plan.AccountId {
			account = &accounts[i]
			break
		}
	}

	if account == nil {
		return nil, fmt.Errorf("could not find active brokerage account %s", plan.AccountId)
	}

	portfolio, err := client.GetPortfolio(ctx, account)
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

	recentTx := make(etrade.RecentTxByTicker)
	for i := range accounts {
		accountHistory, err := client.GetRecentTx(ctx, &accounts[i], daysBack, inScope)
		if err != nil {
			return nil, fmt.Errorf("could not get transactions for account %s: %w", accounts[i].Id, err)
		}

		recentTx.Append(accountHistory)
	}

	var assets []Asset

	for _, assetPlan := range plan.Assets {
		positions := collections.ExtractMapByKeys(portfolio.PositionByTicker, assetPlan.Tickers())
		transactions := collections.ExtractMapByKeys(recentTx, assetPlan.Tickers())
		planned := assignAsset(&assetPlan, positions, transactions, portfolio.TotalValue)

		assets = append(assets, planned)
	}

	// Extra positions should be uncommon, but still technically possible.
	// Aggregate into a single Asset with a nil AssetPlan, which should
	// force Seesaw into liquidating as soon as possible.
	if len(portfolio.PositionByTicker) > 0 {
		unplanned := assignAsset(nil, portfolio.PositionByTicker, recentTx, portfolio.TotalValue)
		assets = append(assets, unplanned)
	}

	return &AccountSnapshot{
		Id:          plan.AccountId,
		CreatedAt:   time.Now(),
		BuyingPower: portfolio.BuyingPower,
		TotalValue:  portfolio.TotalValue,
		Tickers:     inScope.Slice(),
		Assets:      assets,
	}, nil
}

func assignAsset(
	plan *AssetPlan,
	positions map[string]etrade.Position,
	recentTx etrade.RecentTxByTicker,
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
