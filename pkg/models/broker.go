package models

import (
	"context"
	"errors"
)

var ErrBrokerNotConfigured = errors.New("broker is not configured")

type BrokerClient interface {
	Portfolio(ctx context.Context, accountID string, daysBack int, tickers []string) (*Portfolio, error)
	Quote(ctx context.Context, ticker string) (Quote, error)
	QuoteAll(ctx context.Context, tickers []string) (map[string]Quote, error)
	Settings() BrokerSettings
}

type BrokerSettings struct {
	AllowFractionalShares bool
}
