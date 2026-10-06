package trading

import (
	"context"
	"fmt"
	"seesaw/pkg/models"
)

func fetchQuotes(
	ctx context.Context,
	client models.BrokerClient,
	tickers []string,
) (map[string]models.Quote, error) {
	quotes := make(map[string]models.Quote, len(tickers))
	for _, ticker := range tickers {
		quote, err := client.Quote(ctx, ticker)
		if err != nil {
			return nil, fmt.Errorf("quote %s: %w", ticker, err)
		}

		quotes[ticker] = quote
	}
	return quotes, nil
}
