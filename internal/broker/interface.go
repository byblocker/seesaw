package broker

import (
	"context"
	"seesaw/pkg/models"
)

const ETRADE = "ETRADE"

type Account interface {
	AccountID() string
}

type Settings struct {
	AllowFractionalShares bool
}

type Broker interface {
	Settings() Settings

	GetAccounts(ctx context.Context) ([]Account, error)
	GetPortfolio(ctx context.Context, account Account) (*models.Portfolio, error)
	GetRecentTx(ctx context.Context, account Account, daysBack int, tickers []string) (models.RecentTxByTicker, error)
	Quote(ctx context.Context, ticker string) (models.Quote, error)
}
