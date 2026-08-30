package pkg

import (
	"fmt"
	"seesaw/pkg/services"
	"seesaw/pkg/services/etrade"
	"seesaw/pkg/utils"
)

func Rebalance() (any, error) {
	env := services.LoadEnv()

	etrade, err := etrade.NewClient(
		env.ETradeApiKey,
		env.ETradeApiSecret,
		env.ETradeAPIBaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ETrade: %v", err)
	}

	utils.DebugYAML(etrade)

	return nil, nil
}
