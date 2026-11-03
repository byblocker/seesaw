package portfolio

import (
	"context"
	"fmt"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

const transactionHistoryDays = models.WashSaleWindowDays + 1

type Snapshot struct {
	AccountId  string
	CreatedAt  time.Time
	Balances   models.Balances
	Assets     []Asset
	Positions  []Position
	Lots       []LotSnapshot
	TotalValue decimal.Decimal
}

func TakeSnapshot(
	ctx context.Context,
	client models.BrokerClient,
	plan *config.Plan,
) (Snapshot, error) {
	portfolio, err := client.Portfolio(
		ctx,
		plan.AccountId,
		transactionHistoryDays,
		slices.Collect(plan.Tickers()),
	)
	if err != nil {
		return Snapshot{}, fmt.Errorf("failed to retrieve portfolio: %w", err)
	}

	createdAt := time.Now()
	assets, positions, lots, err := groupByAsset(plan.Assets, portfolio, createdAt)
	if err != nil {
		return Snapshot{}, err
	}

	totalValue := portfolio.Balances.CashBalance
	for _, position := range positions {
		asset := &assets[position.AssetIdx]
		value := position.Value(lots)
		asset.Value = asset.Value.Add(value)
		totalValue = totalValue.Add(value)
	}

	for i := range assets {
		assets[i].allocate(totalValue)
	}

	return Snapshot{
		AccountId:  plan.AccountId,
		CreatedAt:  createdAt,
		Balances:   portfolio.Balances,
		Assets:     assets,
		Positions:  positions,
		Lots:       lots,
		TotalValue: totalValue,
	}, nil
}

func (snapshot Snapshot) Simulate(sells []SellProposal, buys []Buy, withdrawal decimal.Decimal) Snapshot {
	cash := snapshot.Balances.CashBalance.Sub(withdrawal)
	changeByTicker := map[string]decimal.Decimal{}
	for _, sell := range sells {
		changeByTicker[sell.Ticker] = changeByTicker[sell.Ticker].Sub(sell.Proceeds)
		cash = cash.Add(sell.Proceeds)
	}
	for _, buy := range buys {
		changeByTicker[buy.Ticker] = changeByTicker[buy.Ticker].Add(buy.EstimatedCost)
		cash = cash.Sub(buy.EstimatedCost)
	}

	simulated := snapshot
	simulated.Balances.CashBalance = cash
	simulated.Assets = slices.Clone(snapshot.Assets)
	simulated.TotalValue = cash
	for _, position := range simulated.Positions {
		asset := &simulated.Assets[position.AssetIdx]
		asset.Value = asset.Value.Add(changeByTicker[position.Ticker])
	}
	for _, asset := range simulated.Assets {
		simulated.TotalValue = simulated.TotalValue.Add(asset.Value)
	}

	for i := range simulated.Assets {
		simulated.Assets[i].allocate(simulated.TotalValue)
	}
	return simulated
}
