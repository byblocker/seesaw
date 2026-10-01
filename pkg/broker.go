package pkg

import (
	"context"
	"seesaw/internal/broker"
	"seesaw/internal/broker/etrade"
	"seesaw/pkg/models"
)

type ETradeConfig = etrade.Config

var ErrBrokerNotConfigured = broker.ErrNotConfigured

func Connect(ctx context.Context, name string) (models.BrokerClient, error) {
	selectedBroker, err := broker.For(name)
	if err != nil {
		return nil, err
	}
	return selectedBroker.Connect(ctx)
}

func ConnectETrade(ctx context.Context, config ETradeConfig) (*etrade.Client, error) {
	return etrade.NewClient(ctx, config)
}
