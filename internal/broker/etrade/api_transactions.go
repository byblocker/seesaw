package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/async"
	"seesaw/internal/broker"
	"seesaw/internal/validate"
	"seesaw/pkg/models"
	"slices"
	"strings"
	"time"
)

const dateFormat = "01022006"

// ETrade's transactionType values for the trades the snapshot tracks; others are skipped.
var transactionTypes = map[string]models.TransactionType{
	"Bought": models.TxBuy,
	"Sold":   models.TxSell,
}

type transactionRow struct {
	AccountID string `json:"accountId"`
	Type      string `json:"transactionType"`
	Date      int64  `json:"transactionDate" validate:"gt=0"`
	Brokerage struct {
		Ticker string `json:"displaySymbol"`
	} `json:"brokerage"`
}

type transactionResponse struct {
	Page struct {
		Transactions []transactionRow `json:"Transaction"`
		NextPage     string           `json:"next"`
	} `json:"TransactionListResponse"`
}

// Fetch every account's supported transactions, for any ticker.
func (client *Client) fetchTransactions(
	ctx context.Context,
	accounts []Account,
	start, end time.Time,
) ([]transactionRow, error) {
	rowsByAccount, err := async.Map(ctx, accounts, func(ctx context.Context, account Account) ([]transactionRow, error) {
		rows, err := client.fetchTransactionsFor(ctx, account, start, end)
		if err != nil {
			return nil, fmt.Errorf("could not get transactions for account %s: %w", account.ID(), err)
		}
		return rows, nil
	})
	if err != nil {
		return nil, err
	}

	return slices.Concat(rowsByAccount...), nil
}

func (client *Client) fetchTransactionsFor(
	ctx context.Context,
	account Account,
	start, end time.Time,
) ([]transactionRow, error) {
	pageURL := client.endpointFor(account, "transactions")
	query := url.Values{
		"startDate": {start.Format(dateFormat)},
		"endDate":   {end.Format(dateFormat)},
	}
	pageURL += "?" + query.Encode()
	firstPageURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("parse transactions endpoint: %w", err)
	}

	var rows []transactionRow
	visitedPages := map[string]bool{}

	for {
		if visitedPages[pageURL] {
			return nil, fmt.Errorf("transaction pagination repeated a page")
		}
		visitedPages[pageURL] = true

		response, err := broker.FetchJSON[transactionResponse](
			ctx,
			client.httpClient,
			pageURL,
		)
		if err != nil {
			return nil, fmt.Errorf("fetch transactions: %w", err)
		}

		for _, tx := range response.Page.Transactions {
			if _, tracked := transactionTypes[tx.Type]; tracked {
				rows = append(rows, tx)
			}
		}

		pageURL = response.Page.NextPage
		if pageURL == "" {
			break
		}
		// The client signs every request, so only follow pages of this endpoint.
		nextPageURL, err := url.Parse(pageURL)
		if err != nil || nextPageURL.Scheme != firstPageURL.Scheme || nextPageURL.Host != firstPageURL.Host || nextPageURL.Path != firstPageURL.Path {
			return nil, fmt.Errorf("transaction next page is outside the transactions endpoint")
		}
	}

	return rows, nil
}

// Validate and convert the transactions for the tickers, skipping the rest.
func parseTransactions(rows []transactionRow, tickers []string) ([]models.Transaction, error) {
	var transactions []models.Transaction
	for _, tx := range rows {
		tx.Brokerage.Ticker = strings.TrimSpace(tx.Brokerage.Ticker)
		if !slices.Contains(tickers, tx.Brokerage.Ticker) {
			continue
		}
		if err := validate.Fields(tx, "transaction."); err != nil {
			return nil, fmt.Errorf("%s transaction for %s in account %s: %w", tx.Type, tx.Brokerage.Ticker, tx.AccountID, err)
		}

		transactions = append(transactions, models.Transaction{
			Ticker: tx.Brokerage.Ticker,
			Type:   transactionTypes[tx.Type],
			Date:   time.UnixMilli(tx.Date),
		})
	}
	return transactions, nil
}
