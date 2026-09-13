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

const (
	TxBuy  = "Bought"
	TxSell = "Sold"
)

type Transaction struct {
	Ticker   string           `json:"displaySymbol"`
	Quantity udecimal.Decimal `json:"quantity"`
	Price    udecimal.Decimal `json:"price"`
}
type Transactions []Transaction

type getTransactionResponse struct {
	TransactionListResponse struct {
		TransactionList []struct {
			AccountID   string      `json:"accountId"`
			Type        string      `json:"transactionType"`
			Transaction Transaction `json:"brokerage"`
		} `json:"Transaction"`
		NextPage string `json:"next"`
	} `json:"TransactionListResponse"`
}

func (client Client) GetRecentTransactionsByTicker(
	ctx context.Context,
	account *AccountDetails,
	daysBack int,
) (map[string]Transactions, error) {
	byTicker := make(map[string]Transactions)

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

		for _, tx := range response.TransactionListResponse.TransactionList {
			ticker := strings.TrimSpace(tx.Transaction.Ticker)

			wrongAccount := tx.AccountID != account.Id
			invalidTicker := ticker == ""
			if wrongAccount || invalidTicker {
				continue
			}

			byTicker[ticker] = append(byTicker[ticker], tx.Transaction)
		}

		nextPage := response.TransactionListResponse.NextPage
		if nextPage == "" {
			break
		}

		endpoint = nextPage
		query = nil
	}

	return byTicker, nil
}
