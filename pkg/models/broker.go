package models

import "context"

type Settings struct {
	AllowFractionalShares bool
}

type Broker interface {
	Settings() Settings

	Portfolio(ctx context.Context, accountID string, daysBack int, tickers []string) (*Portfolio, error)
	Quote(ctx context.Context, ticker string) (Quote, error)
}
