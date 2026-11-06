package etrade

import (
	"context"
	"fmt"
	"log"
	"maps"
	"net/url"
	"seesaw/internal/async"
	"seesaw/internal/broker"
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

func (client *Client) endpointFor(account Account, path string) string {
	return fmt.Sprintf("%s/accounts/%s/%s", client.apiBaseURL, url.PathEscape(account.key), path)
}

func (client *Client) Portfolio(
	ctx context.Context,
	accountID string,
	daysBack int,
	plannedTickers []string,
) (models.Portfolio, error) {
	// We grab all accounts since different accounts may include transactions
	// that could be relevant for tax-loss harvesting and wash-sale avoidance.
	accounts, err := client.fetchAccounts(ctx)
	if err != nil {
		return models.Portfolio{}, err
	}

	accountIdx := slices.IndexFunc(accounts, func(account Account) bool {
		return account.ID() == accountID
	})
	if accountIdx < 0 {
		return models.Portfolio{}, fmt.Errorf("could not find active brokerage account %s", accountID)
	}
	selectedAccount := accounts[accountIdx]

	txEnd := time.Now()
	txStart := txEnd.AddDate(0, 0, -daysBack)
	log.Printf(
		"Checking transactions from %s to %s",
		txStart.Format(time.DateOnly),
		txEnd.Format(time.DateOnly),
	)

	// One timer covers every request below, since concurrent timers would interleave their lines.
	var portfolio models.Portfolio
	var portfolioTickers []string
	var txRows []transactionRow
	stop := broker.StartTimer("Fetching holdings, tax lots, balances, transactions, and quotes")
	group, groupCtx := errgroup.WithContext(ctx)

	// Only the tax lots and quotes wait on the holdings list.
	group.Go(func() error {
		positionsByTicker, err := client.listPositions(groupCtx, selectedAccount)
		if err != nil {
			return err
		}

		heldTickers := slices.Collect(maps.Keys(positionsByTicker))
		portfolioTickers = slices.Compact(slices.Sorted(slices.Values(slices.Concat(plannedTickers, heldTickers))))

		// One request per ticker returns faster than one request listing every ticker.
		group.Go(func() (err error) {
			portfolio.Quotes, err = async.Map(groupCtx, portfolioTickers, client.fetchQuote)
			return err
		})

		lotsByPosition, err := async.Map(groupCtx, slices.Collect(maps.Values(positionsByTicker)), func(ctx context.Context, position positionResponse) ([]models.TaxLot, error) {
			return client.fetchLots(ctx, selectedAccount, position)
		})
		portfolio.Lots = slices.Concat(lotsByPosition...)
		return err
	})

	group.Go(func() (err error) {
		portfolio.Balances, err = client.fetchBalances(groupCtx, selectedAccount)
		return err
	})

	group.Go(func() (err error) {
		txRows, err = client.fetchTransactions(groupCtx, accounts, txStart, txEnd)
		return err
	})

	err = group.Wait()
	if err == nil {
		portfolio.Transactions, err = parseTransactions(txRows, portfolioTickers)
	}
	stop(err)
	if err != nil {
		return models.Portfolio{}, err
	}

	return portfolio, nil
}
