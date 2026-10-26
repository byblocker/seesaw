package models

import (
	"context"
	"errors"
)

var ErrBrokerNotConfigured = errors.New("broker is not configured")

type BrokerClient interface {
	Portfolio(ctx context.Context, accountID string, daysBack int, tickers []string) (*Portfolio, error)
	Settings() BrokerSettings
}

type BrokerSettings struct {
	AllowFractionalShares bool
}
