package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/broker"
	"seesaw/internal/config"
	"seesaw/pkg/models"
)

func ConnectBroker(
	ctx context.Context,
	name string,
	env config.ENV,
) (models.Broker, error) {
	var (
		client models.Broker
		err    error
	)

	switch name {
	case broker.ETRADE:
		client, err = broker.NewETradeClient(
			ctx,
			env.ETradeApiKey, env.ETradeApiSecret, env.ETradeAPIBaseURL,
		)
	default:
		return nil, fmt.Errorf("unsupported broker %q", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", name, err)
	}

	return client, nil
}
