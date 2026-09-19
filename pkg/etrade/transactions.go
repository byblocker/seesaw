package etrade

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"seesaw/internal/httpclient"
	"seesaw/internal/utils"
	"strings"
	"time"

	"github.com/quagmt/udecimal"
)

type RecentTxByTicker map[string]struct {
	Buys  []Transaction
	Sells []Transaction
}

type Transaction struct {
	Ticker   string
	Type     TransactionType
	Quantity udecimal.Decimal
	Price    udecimal.Decimal
}

type TransactionType string

const (
	TxBuy  TransactionType = "Bought"
	TxSell TransactionType = "Sold"
)

type transactionResponse struct {
	Page struct {
		Transactions []struct {
			AccountID string `json:"accountId"`
			Type      string `json:"transactionType"`
			Brokerage struct {
				Ticker   string           `json:"displaySymbol"`
				Quantity udecimal.Decimal `json:"quantity"`
				Price    udecimal.Decimal `json:"price"`
			} `json:"brokerage"`
		} `json:"Transaction"`
		NextPage string `json:"next"`
	} `json:"TransactionListResponse"`
}

func (client *Client) GetRecentTransactions(ctx context.Context, accountID string, daysBack int, tickers []string) (RecentTxByTicker, error) {
	result := make(RecentTxByTicker)
	if len(tickers) == 0 {
		return result, nil
	}

	account, err := client.getAccountDetails(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("resolve account %s: %w", accountID, err)
	}

	inScope := make(map[string]bool, len(tickers))
	for _, ticker := range tickers {
		inScope[ticker] = true
	}

	end := time.Now()
	start := end.AddDate(0, 0, -daysBack)
	endpoint := fmt.Sprintf("%s/accounts/%s/transactions", client.apiBaseURL, url.PathEscape(account.Key))
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
			ticker := strings.TrimSpace(tx.Brokerage.Ticker)
			kind := TransactionType(tx.Type)
			if !inScope[ticker] || (kind != TxBuy && kind != TxSell) {
				continue
			}
			transaction := Transaction{Ticker: ticker, Type: kind, Quantity: tx.Brokerage.Quantity, Price: tx.Brokerage.Price}
			history := result[ticker]
			if kind == TxBuy {
				history.Buys = append(history.Buys, transaction)
			} else {
				history.Sells = append(history.Sells, transaction)
			}
			result[ticker] = history
		}
		endpoint = response.Page.NextPage
		if endpoint == "" {
			break
		}
		query = nil
	}
	return result, nil
}
