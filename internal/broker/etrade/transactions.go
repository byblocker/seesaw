package etrade

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"seesaw/internal/broker"
	"seesaw/internal/collections"
	"seesaw/internal/httpclient"
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

func (client *Client) GetRecentTx(
	ctx context.Context,
	handle broker.Account,
	daysBack int,
	tickers []string,
) (models.RecentTxByTicker, error) {
	end := time.Now()
	start := end.AddDate(0, 0, -daysBack)
	return client.GetTx(ctx, handle, start, end, tickers)
}

func (client *Client) GetTx(
	ctx context.Context,
	handle broker.Account,
	start, end time.Time,
	tickers []string,
) (models.RecentTxByTicker, error) {
	result := make(models.RecentTxByTicker)
	if len(tickers) == 0 {
		return result, nil
	}

	account, ok := handle.(brokerageAccount)
	if !ok || account.key == "" {
		return nil, fmt.Errorf("expected an ETRADE account with a nonempty key")
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

		response, err := httpclient.SendJSONRequest[transactionResponse](
			ctx,
			client.httpClient,
			http.MethodGet,
			endpoint,
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("fetch transactions: %w", err)
		}

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
