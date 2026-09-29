package models

import "context"

type Broker interface {
	Portfolio(ctx context.Context, accountID string, daysBack int, tickers []string) (*Portfolio, error)
	Quote(ctx context.Context, ticker string) (Quote, error)
	Settings() BrokerSettings
}

type BrokerSettings struct {
	AllowFractionalShares bool
}
