package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/async"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"strings"

	"github.com/shopspring/decimal"
)

type quoteResponsePrices struct {
	Ask decimal.Decimal `json:"ask"`
	Bid decimal.Decimal `json:"bid"`
}

type quoteResponse struct {
	QuoteResponse struct {
		QuoteData []struct {
			Product struct {
				Symbol string `json:"symbol"`
			} `json:"Product"`
			All quoteResponsePrices `json:"All"`
		} `json:"QuoteData"`
	} `json:"QuoteResponse"`
}

// Concurrent requests return faster than one request listing every ticker.
func (client *Client) QuoteAll(ctx context.Context, tickers []string) (map[string]models.Quote, error) {
	var timer utils.Timer
	timer.Start(fmt.Sprintf("Fetching quotes for %d ticker(s)", len(tickers)))
	quotes, err := async.MapByKey(ctx, tickers, client.fetchQuote)
	timer.Stop(err)

	return quotes, err
}

func (client *Client) fetchQuote(ctx context.Context, ticker string) (models.Quote, error) {
	ticker = strings.TrimSpace(ticker)
	if ticker == "" {
		return models.Quote{}, fmt.Errorf("invalid ticker: %s", ticker)
	}

	endpoint := fmt.Sprintf("%s/market/quote/%s", client.apiBaseURL, url.PathEscape(ticker))
	endpoint += "?" + url.Values{"detailFlag": {"ALL"}}.Encode()

	response, err := utils.FetchJSON[quoteResponse](
		ctx,
		client.httpClient,
		endpoint,
	)
	if err != nil {
		return models.Quote{}, fmt.Errorf("quote %s: %w", ticker, err)
	}

	for _, quote := range response.QuoteResponse.QuoteData {
		if strings.EqualFold(quote.Product.Symbol, ticker) {
			if !quote.All.Bid.IsPositive() {
				return models.Quote{}, fmt.Errorf("invalid bid price for %s: %s (must be positive)", ticker, quote.All.Bid)
			}

			if !quote.All.Ask.IsPositive() {
				return models.Quote{}, fmt.Errorf("invalid ask price for %s: %s (must be positive)", ticker, quote.All.Ask)
			}

			return models.Quote{Ask: quote.All.Ask, Bid: quote.All.Bid}, nil
		}
	}

	return models.Quote{}, fmt.Errorf("no quote returned for %s", ticker)
}
