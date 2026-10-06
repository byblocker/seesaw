package broker

import (
	"context"
	brokerimpl "seesaw/internal/broker"
	"seesaw/internal/broker/etrade"
	"seesaw/pkg/models"
)

type ETradeConfig = etrade.Config

type ETradeClient = etrade.Client

var ErrNotConfigured = brokerimpl.ErrNotConfigured

func Connect(ctx context.Context, name string) (models.BrokerClient, error) {
	selectedBroker, err := brokerimpl.For(name)
	if err != nil {
		return nil, err
	}
	return selectedBroker.Connect(ctx)
}

func ConnectETrade(ctx context.Context, config ETradeConfig) (*ETradeClient, error) {
	return etrade.NewClient(ctx, config)
}
