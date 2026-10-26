package etrade

import (
	"context"
	"fmt"
	"log"
	"maps"
	"seesaw/internal/async"
	"seesaw/internal/collections"
	"seesaw/internal/utils"
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

	byTicker, err := client.listPositions(ctx, selected)
	if err != nil {
		return nil, err
	}

	positions, err := client.fetchLots(ctx, selected, byTicker)
	if err != nil {
		return nil, err
	}

	balances, err := client.Balances(ctx, selected)
	if err != nil {
		return nil, err
	}

	portfolio := &models.Portfolio{
		Balances:         balances,
		PositionByTicker: positions,
	}

	// Include any held tickers along with the planned tickers
	allTickers := collections.SetFrom(
		slices.Values(tickers),
		maps.Keys(portfolio.PositionByTicker),
	).Slice()

	end := time.Now()
	start := end.AddDate(0, 0, -daysBack)
	log.Printf(
		"Checking transactions from %s to %s",
		start.Format(time.DateOnly),
		end.Format(time.DateOnly),
	)

	stop := utils.StartTimer("Fetching transactions for %d account(s)", len(accounts))
	historyByAccount, err := async.ForKeys(ctx, accounts, func(ctx context.Context, account Account) (models.RecentTxByTicker, error) {
		history, err := client.Transactions(ctx, account, start, end, allTickers)
		if err != nil {
			return nil, fmt.Errorf("could not get transactions for account %s: %w", account.ID(), err)
		}
		return history, nil
	})
	stop(err)
	if err != nil {
		return nil, err
	}

	portfolio.RecentTx = make(models.RecentTxByTicker)
	for _, account := range accounts {
		portfolio.RecentTx.Append(historyByAccount[account])
	}

	return portfolio, nil
}
