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
	"golang.org/x/sync/errgroup"
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

	// Include any held tickers along with the planned tickers
	allTickers := collections.SetFrom(
		slices.Values(tickers),
		maps.Keys(byTicker),
	).Slice()

	end := time.Now()
	start := end.AddDate(0, 0, -daysBack)
	log.Printf(
		"Checking transactions from %s to %s",
		start.Format(time.DateOnly),
		end.Format(time.DateOnly),
	)

	// One timer covers every request below, since concurrent timers would interleave their lines.
	portfolio := &models.Portfolio{}
	stop := utils.StartTimer("Fetching tax lots, balances, transactions, and quotes")
	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() (err error) {
		portfolio.PositionByTicker, err = async.ForValues(groupCtx, byTicker, func(ctx context.Context, pos positionResponse) (models.Position, error) {
			return client.fetchPosition(ctx, selected.key, pos)
		})
		return err
	})

	group.Go(func() (err error) {
		portfolio.Balances, err = client.Balances(groupCtx, selected)
		return err
	})

	group.Go(func() (err error) {
		portfolio.RecentTx, err = client.AllRecentTx(groupCtx, accounts, start, end, allTickers)
		return err
	})

	// Concurrent requests return faster than one request listing every ticker.
	group.Go(func() (err error) {
		portfolio.Quotes, err = async.ForKeys(groupCtx, allTickers, client.fetchQuote)
		return err
	})

	err = group.Wait()
	stop(err)
	if err != nil {
		return nil, err
	}

	return portfolio, nil
}
