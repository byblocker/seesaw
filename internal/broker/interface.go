package broker

import (
	"context"
	"seesaw/pkg/models"
)

type Account interface {
	AccountID() string
}

type Broker interface {
	GetAccounts(ctx context.Context) ([]Account, error)
	GetPortfolio(ctx context.Context, account Account) (*models.Portfolio, error)
	GetRecentTx(ctx context.Context, account Account, daysBack int, tickers []string) (models.RecentTxByTicker, error)
	Quote(ctx context.Context, ticker string) (models.Quote, error)
}
