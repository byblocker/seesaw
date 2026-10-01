package etrade

import (
	"context"
	"fmt"
	"maps"
	"seesaw/internal/collections"
	"seesaw/pkg/models"
	"slices"
	"time"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	httpClient *resty.Client
	apiBaseURL string
}

var _ models.BrokerClient = (*Client)(nil)

func (client *Client) Settings() models.BrokerSettings {
	return models.BrokerSettings{AllowFractionalShares: false}
}

func (client *Client) Portfolio(
	ctx context.Context,
	accountID string,
	daysBack int,
	tickers []string,
) (*models.Portfolio, error) {
	if daysBack <= 0 {
		return nil, fmt.Errorf("transaction lookback must be positive")
	}

	// We grab all accounts since different accounts may include transactions
	// that could be relevant for tax-loss harvesting and wash-sale avoidance.
	accounts, err := client.Accounts(ctx)
	if err != nil {
		return nil, err
	}

	selected, found := collections.Find(
		slices.Values(accounts),
		func(account Account) bool {
			return account.ID() == accountID
		},
	)
	if !found {
		return nil, fmt.Errorf("could not find active brokerage account %s", accountID)
	}

	portfolio, err := client.Holdings(ctx, selected)
	if err != nil {
		return nil, err
	}

	// Include any held tickers along with the planned tickers
	portfolio.Tickers = collections.SetFrom(
		slices.Values(tickers),
		maps.Keys(portfolio.PositionByTicker),
	).Slice()

	end := time.Now()
	start := end.AddDate(0, 0, -daysBack)
	portfolio.RecentTx = make(models.RecentTxByTicker)
	for _, account := range accounts {
		history, err := client.Transactions(ctx, account, start, end, portfolio.Tickers)
		if err != nil {
			return nil, fmt.Errorf("could not get transactions for account %s: %w", account.ID(), err)
		}
		portfolio.RecentTx.Append(history)
	}

	return portfolio, nil
}
