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

const (
	washSaleWindowDays     = 30
	transactionHistoryDays = washSaleWindowDays + 1
)

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
	assets, positions, positionIdxByTicker := newAssets(plan.Assets, portfolio.Lots)
	if err := setQuotes(positions, positionIdxByTicker, portfolio.Quotes); err != nil {
		return Snapshot{}, err
	}
	flagRecentTrades(positions, positionIdxByTicker, portfolio.Transactions, createdAt)
	lots := newLotSnapshots(positions, positionIdxByTicker, portfolio.Lots, createdAt)

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

// The snapshot after the orders and the withdrawal. Its positions and lots are the snapshot's own, unchanged.
func (snapshot Snapshot) Simulate(sells []SellProposal, buys []Buy, withdrawal decimal.Decimal) Snapshot {
	simulated := snapshot
	simulated.Assets = slices.Clone(snapshot.Assets)
	cash := snapshot.Balances.CashBalance.Sub(withdrawal)
	for _, sell := range sells {
		asset := &simulated.Assets[snapshot.Positions[sell.PositionIdx].AssetIdx]
		asset.Value = asset.Value.Sub(sell.Proceeds)
		cash = cash.Add(sell.Proceeds)
	}
	for _, buy := range buys {
		asset := &simulated.Assets[snapshot.Positions[buy.PositionIdx].AssetIdx]
		asset.Value = asset.Value.Add(buy.EstimatedCost)
		cash = cash.Sub(buy.EstimatedCost)
	}

	simulated.Balances.CashBalance = cash
	simulated.TotalValue = cash
	for _, asset := range simulated.Assets {
		simulated.TotalValue = simulated.TotalValue.Add(asset.Value)
	}

	for i := range simulated.Assets {
		simulated.Assets[i].allocate(simulated.TotalValue)
	}
	return simulated
}
