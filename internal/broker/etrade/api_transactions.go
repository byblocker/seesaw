package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/collections"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"time"

	"github.com/shopspring/decimal"
)

type transactionResponse struct {
	Page struct {
		Transactions []struct {
			AccountID string `json:"accountId"`
			Type      string `json:"transactionType"`
			Brokerage struct {
				Ticker   string          `json:"displaySymbol"`
				Quantity decimal.Decimal `json:"quantity"`
				Price    decimal.Decimal `json:"price"`
			} `json:"brokerage"`
		} `json:"Transaction"`
		NextPage string `json:"next"`
	} `json:"TransactionListResponse"`
}

func (client *Client) Transactions(
	ctx context.Context,
	account Account,
	start, end time.Time,
	tickers []string,
) (result models.RecentTxByTicker, err error) {
	result = make(models.RecentTxByTicker)
	if len(tickers) == 0 {
		return result, nil
	}

	if account.key == "" {
		return nil, fmt.Errorf("expected an ETRADE account with a nonempty key")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var timer utils.Timer
	verbose := utils.IsVerbose(ctx)
	pages := 0
	if !verbose {
		timer.Start("Fetching transactions for " + account.maskedID())
		defer func() {
			timer.Stop(err, fmt.Sprintf("%d page(s)", pages))
		}()
	}

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/transactions",
		client.apiBaseURL,
		url.PathEscape(account.key),
	)
	query := url.Values{
		"startDate": {start.Format(utils.MMDDYYYY)},
		"endDate":   {end.Format(utils.MMDDYYYY)},
	}
	endpoint += "?" + query.Encode()

	tickerSet := collections.NewSet(tickers...)
	visitedPages := collections.NewSet[string]()

	for {
		if visitedPages.Contains(endpoint) {
			return nil, fmt.Errorf("transaction pagination repeated a page")
		}
		visitedPages.Add(endpoint)

		if verbose {
			timer.Start(fmt.Sprintf("Fetching transactions for %s, page %d", account.maskedID(), pages+1))
		}
		response, err := utils.FetchJSON[transactionResponse](
			ctx,
			client.httpClient,
			endpoint,
		)
		if verbose {
			timer.Stop(err)
		}
		if err != nil {
			return nil, fmt.Errorf("fetch transactions: %w", err)
		}
		pages++

		for _, tx := range response.Page.Transactions {
			if !tickerSet.Contains(tx.Brokerage.Ticker) || !models.TransactionType(tx.Type).IsSupported() {
				continue
			}

			result.Add(models.Transaction{
				Ticker:   tx.Brokerage.Ticker,
				Type:     models.TransactionType(tx.Type),
				Quantity: tx.Brokerage.Quantity,
				Price:    tx.Brokerage.Price,
			})
		}

		endpoint = response.Page.NextPage
		if endpoint == "" {
			break
		}
	}

	return result, nil
}
