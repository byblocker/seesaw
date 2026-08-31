package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/config"
	"seesaw/internal/etrade"
	"seesaw/internal/utils"
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

	portfolio, err := etrade.GetPortfolio(ctx, accountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve portfolio positions")
	}

	utils.DebugYAML(portfolio)

	return nil, nil
}
