package trading

import (
	"fmt"
	"log"
	"seesaw/internal/rebalancer"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
)

func Rebalance(
	snapshot portfolio.Snapshot,
	client models.BrokerClient,
	settings config.Settings,
) (*models.RebalancePlan, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	if !snapshot.TotalValue.IsPositive() {
		return nil, fmt.Errorf("empty or negative portfolio")
	}

	settings.Log(snapshot.TotalValue, client.Settings().AllowFractionalShares)
	r := rebalancer.New(settings)

	log.Printf("Evaluating %d asset(s) for tax-loss harvesting", len(snapshot.Assets))
	var sells []models.SellProposal
	var requests []rebalancer.BuyRequest
	for _, asset := range snapshot.Assets {
		assetSells, assetRequests := r.RebalanceAsset(asset, snapshot.CreatedAt)
		sells = append(sells, assetSells...)
		requests = append(requests, assetRequests...)
	}

	buys, err := r.SizeBuys(client, snapshot, sells, requests)
	if err != nil {
		return nil, err
	}

	log.Printf("Rebalance complete: %d buy(s), %d sell(s)", len(buys), len(sells))
	return &models.RebalancePlan{Buys: buys, Sells: sells}, nil
}
