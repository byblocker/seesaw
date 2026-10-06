package portfolio

import (
	"context"
	"fmt"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"slices"
	"time"
)

const washSaleWindowDays = 31

type Snapshot struct {
	Id        string
	CreatedAt time.Time
	Portfolio *models.Portfolio
	Assets    []Asset
}

func TakeSnapshot(
	ctx context.Context,
	client models.BrokerClient,
	plan *config.Plan,
) (*Snapshot, error) {
	portfolio, err := client.Portfolio(
		ctx,
		plan.AccountId,
		washSaleWindowDays,
		slices.Collect(plan.Tickers()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve portfolio: %w", err)
	}

	assets := groupByAsset(plan.Assets, portfolio)

	return &Snapshot{
		Id:        plan.AccountId,
		CreatedAt: time.Now(),
		Portfolio: portfolio,
		Assets:    assets,
	}, nil
}
