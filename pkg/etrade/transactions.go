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

func (r RecentTxByTicker) Append(other RecentTxByTicker) {
	for ticker, history := range other {
		combined := r[ticker]
		combined.Buys = append(combined.Buys, history.Buys...)
		combined.Sells = append(combined.Sells, history.Sells...)
		r[ticker] = combined
	}
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

func (client *Client) GetRecentTx(
	ctx context.Context,
	account *AccountDetails,
	daysBack int,
	tickers collections.Set[string],
) (RecentTxByTicker, error) {
	end := time.Now()
	start := end.AddDate(0, 0, -daysBack)
	return client.GetTx(ctx, account, start, end, tickers)
}

func (client *Client) GetTx(
	ctx context.Context,
	account *AccountDetails,
	start, end time.Time,
	tickers collections.Set[string],
) (RecentTxByTicker, error) {
	result := make(RecentTxByTicker)
	if len(tickers) == 0 {
		return result, nil
	}

	if account == nil || account.Key == "" {
		return nil, fmt.Errorf("account details with a nonempty key are required")
	}

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/transactions",
		client.apiBaseURL,
		url.PathEscape(account.Key),
	)
	query := url.Values{
		"startDate": {start.Format(utils.MMDDYYYY)},
		"endDate":   {end.Format(utils.MMDDYYYY)},
	}
	endpoint += "?" + query.Encode()

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
	}

	return result, nil
}
