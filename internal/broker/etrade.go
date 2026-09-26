package broker

import (
	"context"
	"fmt"
	"maps"
	"seesaw/internal/broker/etrade"
	"seesaw/internal/collections"
	"seesaw/pkg/models"
	"slices"
	"time"
)

const ETRADE = "ETRADE"

type ETradeClient struct {
	api *etrade.Client
}

var _ models.Broker = (*ETradeClient)(nil)

func NewETradeClient(ctx context.Context, key, secret, baseURL string) (*ETradeClient, error) {
	client, err := etrade.Connect(ctx, key, secret, baseURL)
	if err != nil {
		return nil, err
	}
	return &ETradeClient{api: client}, nil
}

func (client *ETradeClient) Settings() models.Settings {
	return models.Settings{AllowFractionalShares: false}
}

func (client *ETradeClient) Quote(ctx context.Context, ticker string) (models.Quote, error) {
	return client.api.Quote(ctx, ticker)
}

func (client *ETradeClient) Portfolio(ctx context.Context, accountID string, daysBack int, tickers []string) (*models.Portfolio, error) {
	if daysBack <= 0 {
		return nil, fmt.Errorf("transaction lookback must be positive")
	}

	accounts, err := client.api.Accounts(ctx)
	if err != nil {
		return nil, err
	}

	var selected *etrade.Account
	for i := range accounts {
		if accounts[i].ID() == accountID {
			selected = &accounts[i]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("could not find active brokerage account %s", accountID)
	}

	portfolio, err := client.api.Holdings(ctx, *selected)
	if err != nil {
		return nil, err
	}

	portfolio.Tickers = collections.SetFrom(slices.Values(tickers), maps.Keys(portfolio.PositionByTicker)).Slice()
	end := time.Now()
	start := end.AddDate(0, 0, -daysBack)
	portfolio.RecentTx = make(models.RecentTxByTicker)
	for _, account := range accounts {
		history, err := client.api.Transactions(ctx, account, start, end, portfolio.Tickers)
		if err != nil {
			return nil, fmt.Errorf("could not get transactions for account %s: %w", account.ID(), err)
		}
		portfolio.RecentTx.Append(history)
	}
	return portfolio, nil
}
