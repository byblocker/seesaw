package etrade

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"seesaw/internal/collections"
	"seesaw/internal/httpclient"
	"seesaw/internal/utils"
	"time"

	"github.com/shopspring/decimal"
)

type RecentTxByTicker map[string]struct {
	Buys  []Transaction
	Sells []Transaction
}

func (r RecentTxByTicker) Add(transaction Transaction) {
	history := r[transaction.Ticker]

	switch transaction.Type {
	case TxBuy:
		history.Buys = append(history.Buys, transaction)
	case TxSell:
		history.Sells = append(history.Sells, transaction)
	default:
		return
	}

	r[transaction.Ticker] = history
}

type Transaction struct {
	Ticker   string
	Type     TransactionType
	Quantity decimal.Decimal
	Price    decimal.Decimal
}

type TransactionType string

const (
	TxBuy  TransactionType = "Bought"
	TxSell TransactionType = "Sold"
)

func (t TransactionType) IsSupported() bool {
	return t == TxBuy || t == TxSell
}

type transactionResponse struct {
	Page struct {
		Transactions []struct {
			AccountID string          `json:"accountId"`
			Type      TransactionType `json:"transactionType"`
			Brokerage struct {
				Ticker   string          `json:"displaySymbol"`
				Quantity decimal.Decimal `json:"quantity"`
				Price    decimal.Decimal `json:"price"`
			} `json:"brokerage"`
		} `json:"Transaction"`
		NextPage string `json:"next"`
	} `json:"TransactionListResponse"`
}

func (client *Client) GetRecentTransactions(
	ctx context.Context,
	accountID string,
	daysBack int,
	tickers collections.Set[string],
) (RecentTxByTicker, error) {
	result := make(RecentTxByTicker)
	if len(tickers) == 0 {
		return result, nil
	}

	account, err := client.getAccountDetails(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("resolve account %s: %w", accountID, err)
	}

	end := time.Now()
	start := end.AddDate(0, 0, -daysBack)
	endpoint := fmt.Sprintf(
		"%s/accounts/%s/transactions",
		client.apiBaseURL,
		url.PathEscape(account.Key),
	)
	query := url.Values{
		"startDate": {start.Format(utils.MMDDYYYY)},
		"endDate":   {end.Format(utils.MMDDYYYY)},
	}

	for {
		response, err := httpclient.SendJSONRequest[transactionResponse](
			ctx,
			client.httpClient,
			http.MethodGet,
			endpoint,
			nil,
			query,
		)
		if err != nil {
			return nil, fmt.Errorf("fetch transactions: %w", err)
		}

		for _, tx := range response.Page.Transactions {
			if !tickers.Contains(tx.Brokerage.Ticker) || !tx.Type.IsSupported() {
				continue
			}

			result.Add(Transaction{
				Ticker:   tx.Brokerage.Ticker,
				Type:     tx.Type,
				Quantity: tx.Brokerage.Quantity,
				Price:    tx.Brokerage.Price,
			})
		}

		endpoint = response.Page.NextPage
		if endpoint == "" {
			break
		}

		query = nil
	}

	return result, nil
}
