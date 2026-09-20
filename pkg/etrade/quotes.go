package etrade

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"seesaw/internal/httpclient"
	"strings"

	"github.com/shopspring/decimal"
)

type Quote struct {
	Ask decimal.Decimal `json:"ask"`
	Bid decimal.Decimal `json:"bid"`
}

type quoteResponse struct {
	QuoteResponse struct {
		QuoteData []struct {
			Product struct {
				Symbol string `json:"symbol"`
			} `json:"Product"`
			All Quote `json:"All"`
		} `json:"QuoteData"`
	} `json:"QuoteResponse"`
}

func (client *Client) Quote(ctx context.Context, ticker string) (Quote, error) {
	ticker = strings.TrimSpace(ticker)
	if ticker == "" {
		return Quote{}, fmt.Errorf("invalid ticker: %s", ticker)
	}

	endpoint := fmt.Sprintf("%s/market/quote/%s", client.apiBaseURL, url.PathEscape(ticker))
	response, err := httpclient.SendJSONRequest[quoteResponse](
		ctx,
		client.httpClient,
		http.MethodGet,
		endpoint,
		nil,
		url.Values{"detailFlag": {"ALL"}},
	)
	if err != nil {
		return Quote{}, err
	}

	for _, quote := range response.QuoteResponse.QuoteData {
		if strings.EqualFold(quote.Product.Symbol, ticker) {
			if !quote.All.Bid.IsPositive() {
				return Quote{}, fmt.Errorf("invalid bid price for %s: %s (must be positive)", ticker, quote.All.Bid)
			}

			if !quote.All.Ask.IsPositive() {
				return Quote{}, fmt.Errorf("invalid ask price for %s: %s (must be positive)", ticker, quote.All.Ask)
			}

			return quote.All, nil
		}
	}

	return Quote{}, fmt.Errorf("no quote returned for %s", ticker)
}
