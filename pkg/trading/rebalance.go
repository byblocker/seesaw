package trading

import (
	"context"
	"fmt"
	"log"
	"seesaw/internal/rebalancer"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
)

func Rebalance(
	ctx context.Context,
	snapshot portfolio.Snapshot,
	client models.BrokerClient,
	settings config.RebalanceSettings,
) (*models.RebalancePlan, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	if !snapshot.TotalValue.IsPositive() {
		return nil, fmt.Errorf("empty or negative portfolio")
	}

	r := rebalancer.New(settings)

	log.Printf("Evaluating %d asset(s) for tax-loss harvesting", len(snapshot.Assets))
	var sells []models.SellProposal
	var requests []rebalancer.BuyRequest
	for _, asset := range snapshot.Assets {
		sells, buyRequests := r.RebalanceAsset(ctx, asset, snapshot.CreatedAt)
		sells = append(sells, sells...)
		requests = append(requests, buyRequests...)
	}

	buys, remainingBalance, err := r.SizeBuys(ctx, client, snapshot, sells, requests)
	if err != nil {
		return nil, err
	}

	log.Printf("Rebalance complete: %d buy(s), %d sell(s)", len(buys), len(sells))
	return &models.RebalancePlan{Buys: buys, Sells: sells, RemainingBalance: remainingBalance}, nil
}
