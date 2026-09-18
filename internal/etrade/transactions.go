package etrade

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"seesaw/internal"
	"seesaw/internal/utils"
	"strings"
	"time"

	"github.com/quagmt/udecimal"
)

type TransactionType string

const (
	TxBuy  TransactionType = "Bought"
	TxSell TransactionType = "Sold"
)

type Transaction struct {
	Ticker   string
	Type     TransactionType
	Quantity udecimal.Decimal
	Price    udecimal.Decimal
}

type RecentTxByTicker map[string]struct {
	Buys  []Transaction
	Sells []Transaction
}

type getTransactionResponse struct {
	Page struct {
		Transactions []struct {
			AccountID string          `json:"accountId"`
			Type      TransactionType `json:"transactionType"`
			Brokerage struct {
				Ticker   string           `json:"displaySymbol"`
				Quantity udecimal.Decimal `json:"quantity"`
				Price    udecimal.Decimal `json:"price"`
			} `json:"brokerage"`
		} `json:"Transaction"`
		NextPage string `json:"next"`
	} `json:"TransactionListResponse"`
}

func (client Client) GetRecentTransactions(
	ctx context.Context,
	account *AccountDetails,
	daysBack int,
	tickers internal.Set[string],
) (RecentTxByTicker, error) {
	transactions := make(RecentTxByTicker)
	if len(tickers) == 0 {
		return transactions, nil
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -daysBack)
	log.Printf(
		"Fetching transactions from %s to today . . .",
		startDate.Format(utils.YYYY_MM_DD),
	)

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/transactions",
		client.APIBaseUrl, account.Key,
	)

	query := url.Values{
		"startDate": {startDate.Format(utils.MMDDYYYY)},
		"endDate":   {endDate.Format(utils.MMDDYYYY)},
	}

	for {
		response, err := internal.SendJSONRequest[getTransactionResponse](
			ctx, client.httpClient, http.MethodGet, endpoint, nil, query,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch transactions: %w", err)
		}

		for _, tx := range response.Page.Transactions {
			ticker := strings.TrimSpace(tx.Brokerage.Ticker)
			if ticker == "" || !(tx.Type == TxBuy || tx.Type == TxSell) {
				continue
			}
			if !tickers.Contains(ticker) {
				continue
			}

			transaction := Transaction{
				Ticker:   ticker,
				Type:     tx.Type,
				Price:    tx.Brokerage.Price,
				Quantity: tx.Brokerage.Quantity,
			}
			history := transactions[ticker]
			if tx.Type == TxBuy {
				history.Buys = append(history.Buys, transaction)
			} else {
				history.Sells = append(history.Sells, transaction)
			}
			transactions[ticker] = history
		}

		nextPage := response.Page.NextPage
		if nextPage == "" {
			break
		}

		endpoint = nextPage
		query = nil
	}

	return transactions, nil
}
