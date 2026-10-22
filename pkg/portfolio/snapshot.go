package portfolio

import (
	"context"
	"fmt"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"slices"
	"time"
)

const transactionHistoryDays = models.WashSaleWindowDays + 1

type Snapshot struct {
	Id        string
	CreatedAt time.Time
	Portfolio *models.Portfolio
	Assets    []Asset
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

	for i := range assets {
		asset := &assets[i]
		for _, ticker := range asset.Tickers {
			position := newPosition(ticker, portfolio, quotes[ticker], createdAt)
			asset.Positions = append(asset.Positions, position)
		}
	}

	return &Snapshot{
		Id:        plan.AccountId,
		CreatedAt: createdAt,
		Portfolio: portfolio,
		Assets:    assets,
	}, nil
}
