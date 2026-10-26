package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/async"
	"seesaw/internal/collections"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const dateFormat = "01022006"

type transactionResponse struct {
	Page struct {
		Transactions []struct {
			AccountID string `json:"accountId"`
			Type      string `json:"transactionType"`
			Date      int64  `json:"transactionDate" validate:"gt=0"`
			Brokerage struct {
				Ticker   string          `json:"displaySymbol"`
				Quantity decimal.Decimal `json:"quantity"`
				Price    decimal.Decimal `json:"price"`
			} `json:"brokerage"`
		} `json:"Transaction"`
		NextPage string `json:"next"`
	} `json:"TransactionListResponse"`
}

func (client *Client) AllRecentTx(
	ctx context.Context,
	accounts []Account,
	start, end time.Time,
	tickers []string,
) (models.RecentTxByTicker, error) {
	byAccount, err := async.ForKeys(ctx, accounts, func(ctx context.Context, account Account) (models.RecentTxByTicker, error) {
		history, err := client.Transactions(ctx, account, start, end, tickers)
		if err != nil {
			return nil, fmt.Errorf("could not get transactions for account %s: %w", account.ID(), err)
		}
		return history, nil
	})
	if err != nil {
		return nil, err
	}

	merged := make(models.RecentTxByTicker)
	for _, account := range accounts {
		merged.Append(byAccount[account])
	}
	return merged, nil
}

func (client *Client) Transactions(
	ctx context.Context,
	account Account,
	start, end time.Time,
	tickers []string,
) (models.RecentTxByTicker, error) {
	result := make(models.RecentTxByTicker)
	if len(tickers) == 0 {
		return result, nil
	}

	if account.key == "" {
		return nil, fmt.Errorf("expected an ETRADE account with a nonempty key")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/transactions",
		client.apiBaseURL,
		url.PathEscape(account.key),
	)
	query := url.Values{
		"startDate": {start.Format(dateFormat)},
		"endDate":   {end.Format(dateFormat)},
	}
	endpoint += "?" + query.Encode()
	firstPage, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse transactions endpoint: %w", err)
	}

	tickerSet := collections.NewSet(tickers...)
	visitedPages := collections.NewSet[string]()

	for {
		if visitedPages.Contains(endpoint) {
			return nil, fmt.Errorf("transaction pagination repeated a page")
		}
		visitedPages.Add(endpoint)

		response, err := utils.FetchJSON[transactionResponse](
			ctx,
			client.httpClient,
			endpoint,
		)
		if err != nil {
			return nil, fmt.Errorf("fetch transactions: %w", err)
		}

		for _, tx := range response.Page.Transactions {
			tx.Brokerage.Ticker = strings.TrimSpace(tx.Brokerage.Ticker)
			if !tickerSet.Contains(tx.Brokerage.Ticker) || !models.TransactionType(tx.Type).IsSupported() {
				continue
			}
			if err := utils.ValidateFields(tx, "transaction."); err != nil {
				return nil, fmt.Errorf("%s transaction for %s: %w", tx.Type, tx.Brokerage.Ticker, err)
			}

			result.Add(models.Transaction{
				Ticker:   tx.Brokerage.Ticker,
				Type:     models.TransactionType(tx.Type),
				Date:     time.UnixMilli(tx.Date),
				Quantity: tx.Brokerage.Quantity,
				Price:    tx.Brokerage.Price,
			})
		}

		endpoint = response.Page.NextPage
		if endpoint == "" {
			break
		}
		// The client signs every request, so only follow pages of this endpoint.
		nextPage, err := url.Parse(endpoint)
		if err != nil || nextPage.Scheme != firstPage.Scheme || nextPage.Host != firstPage.Host || nextPage.Path != firstPage.Path {
			return nil, fmt.Errorf("transaction next page is outside the transactions endpoint")
		}
	}

	return result, nil
}
