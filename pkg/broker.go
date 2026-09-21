package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/broker"
	"seesaw/internal/broker/etrade"
	"seesaw/internal/config"
)

func ConnectBroker(
	ctx context.Context,
	name string,
	env config.ENV,
) (broker.Broker, error) {
	var (
		client broker.Broker
		err    error
	)

	switch name {
	case broker.ETRADE:
		client, err = etrade.NewClient(
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
