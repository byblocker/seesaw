package pkg

import (
	"context"
	"seesaw/internal/broker"
	"seesaw/internal/broker/etrade"
	"seesaw/pkg/models"
)

type ETradeConfig = etrade.Config
type BrokerProvider = broker.Provider

var ErrBrokerNotConfigured = broker.ErrNotConfigured

func Provider(name string) (BrokerProvider, error) {
	return broker.For(name)
}

func Connect(ctx context.Context, name string) (models.Broker, error) {
	provider, err := Provider(name)
	if err != nil {
		return nil, err
	}
	return provider.Connect(ctx)
}

func ConnectETrade(ctx context.Context, config ETradeConfig) (*etrade.Client, error) {
	return etrade.NewClient(ctx, config)
}
