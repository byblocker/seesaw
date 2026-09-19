package etrade

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"seesaw/internal/httpclient"
	"strings"

	"github.com/quagmt/udecimal"
)

type quoteResponse struct {
	QuoteResponse struct {
		QuoteData []struct {
			Product struct {
				Symbol string `json:"symbol"`
			} `json:"Product"`
			All struct {
				Ask udecimal.Decimal `json:"ask"`
			} `json:"All"`
		} `json:"QuoteData"`
	} `json:"QuoteResponse"`
}

func (client *Client) Quote(ctx context.Context, ticker string) (udecimal.Decimal, error) {
	ticker = strings.TrimSpace(ticker)
	if ticker == "" {
		return udecimal.Zero, fmt.Errorf("invalid ticker: %s", ticker)
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
		return udecimal.Zero, err
	}
	for _, quote := range response.QuoteResponse.QuoteData {
		matchFound := strings.EqualFold(quote.Product.Symbol, ticker)
		validPrice := quote.All.Ask.GreaterThan(udecimal.Zero)

		if matchFound && validPrice {
			return quote.All.Ask, nil
		}
	}

	return udecimal.Zero, fmt.Errorf("no ask price returned for %s", ticker)
}
