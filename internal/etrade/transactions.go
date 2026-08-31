package etrade

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"seesaw/internal"
	"seesaw/internal/utils"
	"time"
)

const (
	TxBuy  = "Bought"
	TxSell = "Sold"
)

type RecentTxByTicker map[string]RecentTx

type RecentTx struct {
	BoughtWithinWindow bool
	SoldWithinWindow   bool
}

type getTransactionResponse struct {
	TransactionListResponse struct {
		TransactionList []struct {
			AccountID string `json:"accountId"`
			Type      string `json:"transactionType"`
			Product   struct {
				Ticker string `json:"displaySymbol"`
			} `json:"brokerage"`
		} `json:"Transaction"`
		NextPage string `json:"next"`
	} `json:"TransactionListResponse"`
}

func (client Client) GetRecentTransactions(
	ctx context.Context,
	accountId, accountKey string,
	daysBack int,
) (RecentTxByTicker, error) {
	recentTxByTicker := make(RecentTxByTicker)

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -daysBack)
	log.Printf(
		"Fetching transactions from %s to today . . .",
		startDate.Format(utils.YYYY_MM_DD),
	)

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/transactions",
		client.APIBaseUrl, accountKey,
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

		for _, tx := range response.TransactionListResponse.TransactionList {
			wrongAccount := tx.AccountID != accountId
			invalidTicker := tx.Product.Ticker == "" || tx.Product.Ticker == " "

			if wrongAccount || invalidTicker {
				continue
			}

			recentTx := recentTxByTicker[tx.Product.Ticker]
			switch tx.Type {
			case TxBuy:
				recentTx.BoughtWithinWindow = true
			case TxSell:
				recentTx.SoldWithinWindow = true
			}

			recentTxByTicker[tx.Product.Ticker] = recentTx
		}

		nextPage := response.TransactionListResponse.NextPage
		if nextPage == "" {
			break
		}

		endpoint = nextPage
		query = nil
	}

	return recentTxByTicker, nil
}
