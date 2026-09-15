package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/config"
	"seesaw/internal/etrade"
	"seesaw/internal/utils"
	"time"

	"github.com/quagmt/udecimal"
)

type Asset struct {
	Plan      *AssetPlan
	Positions []etrade.Position

	Allocation udecimal.Decimal
	AllocDrift udecimal.Decimal
}

type AccountSnapshot struct {
	Id          string
	CreatedAt   time.Time
	BuyingPower udecimal.Decimal
	Assets      []Asset
	RecentTx    []etrade.Transaction
}

func SnapshotPortfolio(plan *PortfolioPlan, daysBack int) (*AccountSnapshot, error) {
	ctx := context.Background()

	env, err := config.LoadEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load .env: %v", err)
	}

	etrade, err := etrade.NewClient(
		ctx,
		env.ETradeApiKey,
		env.ETradeApiSecret,
		env.ETradeAPIBaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ETrade: %v", err)
	}

	account, err := etrade.GetAccountDetails(ctx, plan.AccountId)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account key: %v", err)
	}

	portfolio, err := etrade.GetPortfolio(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve portfolio positions")
	}

	if daysBack <= 0 {
		daysBack = config.WashSaleWindowDays
	}

	recentTx, err := etrade.GetRecentTransactions(ctx, account, daysBack)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account transactions")
	}

	snapshot := AccountSnapshot{
		Id:        plan.AccountId,
		CreatedAt: time.Now(),
		Assets:    assignToAssets(plan, portfolio.PositionByTicker),
		RecentTx:  recentTx,
	}

	utils.DebugYAML(snapshot)

	return &snapshot, nil
}

func assignToAssets(
	plan *PortfolioPlan,
	posByTicker map[string]etrade.Position,
) []Asset {
	var assets []Asset

	for _, assetPlan := range plan.Assets {
		var positions []etrade.Position
		var allocation udecimal.Decimal

		appendIfExists := func(ticker string) {
			if pos, exists := posByTicker[ticker]; exists {
				positions = append(positions, pos)
				allocation = allocation.Add(pos.Allocation)
				delete(posByTicker, ticker)
			}
		}

		appendIfExists(assetPlan.PrimaryTicker)
		appendIfExists(assetPlan.AltTicker)

		assets = append(assets, Asset{
			Plan:       &assetPlan,
			Positions:  positions,
			Allocation: allocation,
		})
	}

	hasExtraPos := len(posByTicker) > 0
	if hasExtraPos {
		var extraAlloc udecimal.Decimal
		extraPositions := make([]etrade.Position, 0, len(posByTicker))

		for _, pos := range posByTicker {
			extraPositions = append(extraPositions, pos)
			extraAlloc = extraAlloc.Add(pos.Allocation)
		}

		assets = append(assets, Asset{
			Positions:  extraPositions,
			Allocation: extraAlloc,
		})
	}

	return assets
}
