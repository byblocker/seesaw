package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/config"
	"seesaw/internal/etrade"
)

func GetPortfolio(plan *config.PortfolioPlan) (any, error) {
	ctx := context.Background()

	env := config.LoadEnv()

	etrade, err := etrade.NewClient(
		ctx,
		env.ETradeApiKey,
		env.ETradeApiSecret,
		env.ETradeAPIBaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ETrade: %v", err)
	}

	accountKey, err := etrade.GetAccountKey(ctx, plan.AccountId)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account key: %v", err)
	}
	_ = accountKey

	return nil, nil
}
