package etrade

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"seesaw/internal/httpclient"
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

func (client *Client) Quote(ctx context.Context, ticker string) (models.Quote, error) {
	ticker = strings.TrimSpace(ticker)
	if ticker == "" {
		return models.Quote{}, fmt.Errorf("invalid ticker: %s", ticker)
	}

	endpoint := fmt.Sprintf("%s/market/quote/%s", client.apiBaseURL, url.PathEscape(ticker))
	endpoint += "?" + url.Values{"detailFlag": {"ALL"}}.Encode()
	response, err := httpclient.SendJSONRequest[quoteResponse](
		ctx,
		client.httpClient,
		http.MethodGet,
		endpoint,
		nil,
	)
	if err != nil {
		return models.Quote{}, err
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
