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
	tradeDate := snapshot.CreatedAt

	log.Printf("Fetching quotes for %d ticker(s)", len(snapshot.Portfolio.AllTickers))
	quotes, err := client.QuoteAll(ctx, snapshot.Portfolio.AllTickers)
	if err != nil {
		return nil, err
	}

	holdingsByAsset, err := snapshot.HoldingsByAsset(quotes, tradeDate)
	if err != nil {
		return nil, err
	}

	log.Printf("Evaluating %d asset(s) for tax-loss harvesting", len(snapshot.Assets))
	var sells []models.SellProposal
	var requests []rebalancer.BuyRequest
	for i, asset := range snapshot.Assets {
		assetSells, assetRequests := r.RebalanceAsset(ctx, asset, holdingsByAsset[i], quotes, snapshot.Portfolio.RecentTx, tradeDate)
		sells = append(sells, assetSells...)
		requests = append(requests, assetRequests...)
	}

	buys, remainingBalance, err := r.SizeBuys(ctx, client, snapshot, sells, requests)
	if err != nil {
		return nil, err
	}

	log.Printf("Rebalance complete: %d buy(s), %d sell(s)", len(buys), len(sells))
	return &models.RebalancePlan{Buys: buys, Sells: sells, RemainingBalance: remainingBalance}, nil
}
