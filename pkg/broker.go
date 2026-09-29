package pkg

import (
	"context"
	"fmt"
	"os"
	"seesaw/internal/broker"
	"seesaw/pkg/models"

	"github.com/joho/godotenv"
)

func ConnectFromENV(ctx context.Context, name string) (models.Broker, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not load .env: %v", err)
	}

	switch name {
	case broker.ETRADE:
		return ConnectETrade(ctx, broker.ETradeConfig{
			APIBaseURL: os.Getenv("ETRADE_API_BASE_URL"),
			APIKey:     os.Getenv("ETRADE_API_KEY"),
			APISecret:  os.Getenv("ETRADE_API_SECRET"),
		})
	default:
		return nil, fmt.Errorf("unsupported broker %q", name)
	}
}

func ConnectETrade(
	ctx context.Context,
	config broker.ETradeConfig,
) (*broker.ETradeClient, error) {
	client, err := broker.NewETradeClient(ctx, config)
	if err != nil {
		return nil, err
	}
	return client, nil
}
