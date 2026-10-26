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
	Id         string
	CreatedAt  time.Time
	Balances   models.Balances
	Assets     []Asset
	TotalValue decimal.Decimal
}

func TakeSnapshot(
	ctx context.Context,
	client models.BrokerClient,
	plan *config.Plan,
) (*Snapshot, error) {
	portfolio, err := client.Portfolio(
		ctx,
		plan.AccountId,
		transactionHistoryDays,
		slices.Collect(plan.Tickers()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve portfolio: %w", err)
	}

	createdAt := time.Now()
	assets := groupByAsset(plan.Assets, portfolio)

	var tickers []string
	for _, asset := range assets {
		tickers = append(tickers, asset.Tickers...)
	}

	quotes, err := client.QuoteAll(ctx, tickers)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve quotes: %w", err)
	}

	totalValue := portfolio.Balances.CashBalance
	for i := range assets {
		asset := &assets[i]
		for _, ticker := range asset.Tickers {
			position := newPosition(ticker, portfolio, quotes[ticker], createdAt)
			if position.IsHeld() && !position.Quote.Bid.IsPositive() {
				return nil, fmt.Errorf("missing bid price for %s", ticker)
			}
			asset.Positions = append(asset.Positions, position)
			asset.Value = asset.Value.Add(position.Value())
		}
		totalValue = totalValue.Add(asset.Value)
	}

	for i := range assets {
		assets[i].allocate(totalValue)
	}

	return &Snapshot{
		Id:         plan.AccountId,
		CreatedAt:  createdAt,
		Balances:   portfolio.Balances,
		Assets:     assets,
		TotalValue: totalValue,
	}, nil
}

func (snapshot Snapshot) Simulate(sells []models.SellProposal, buys []models.Buy, withdrawal decimal.Decimal) Snapshot {
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
	for i := range simulated.Assets {
		asset := &simulated.Assets[i]
		for _, ticker := range asset.Tickers {
			asset.Value = asset.Value.Add(changeByTicker[ticker])
		}
		simulated.TotalValue = simulated.TotalValue.Add(asset.Value)
	}

	for i := range simulated.Assets {
		simulated.Assets[i].allocate(simulated.TotalValue)
	}
	return simulated
}
