package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"strings"

	"github.com/shopspring/decimal"
)

type quoteResponsePrices struct {
	Ask decimal.Decimal `json:"ask" validate:"gt=0"`
	Bid decimal.Decimal `json:"bid" validate:"gt=0"`
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
			if err := utils.ValidateFields(quote.All, ""); err != nil {
				return models.Quote{}, fmt.Errorf("quote %s: %w", ticker, err)
			}

			return models.Quote{Ask: quote.All.Ask, Bid: quote.All.Bid}, nil
		}
	}

	return models.Quote{}, fmt.Errorf("no quote returned for %s", ticker)
}
