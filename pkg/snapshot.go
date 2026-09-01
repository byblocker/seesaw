package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/config"
	"seesaw/internal/etrade"
	"seesaw/internal/utils"
)

type PortfolioSnapshot struct {
	Id               string
	Portfolio        *etrade.Portfolio
	RecentTxByTicker etrade.RecentTxByTicker
}

func SnapshotPortfolio(plan *config.PortfolioPlan, daysBack int) (*PortfolioSnapshot, error) {
	ctx := context.Background()

	env, err := config.LoadEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load .env: %v", err)
	}

	etrade, err := etrade.NewClient(
		ctx,
		env.ETradeApiKey,
		env.ETradeApiSecret,
		env.ETradeAPIBaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ETrade: %v", err)
	}

	account, err := etrade.GetAccountDetails(ctx, plan.AccountId)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account key: %v", err)
	}

	portfolio, err := etrade.GetPortfolio(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve portfolio positions")
	}

	if daysBack <= 0 {
		daysBack = config.DefaultTransactionWindowDays
	}

	recentTxByTicker, err := etrade.GetRecentTransactions(ctx, account, daysBack)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account transactions")
	}

	snapshot := PortfolioSnapshot{
		Id:               plan.AccountId,
		Portfolio:        portfolio,
		RecentTxByTicker: recentTxByTicker,
	}

	utils.DebugYAML(snapshot)

	return &snapshot, nil
}
