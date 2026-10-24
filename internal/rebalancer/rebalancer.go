package rebalancer

import (
	"context"
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"time"
)

type Rebalancer struct {
	settings config.RebalanceSettings
}

func New(settings config.RebalanceSettings) *Rebalancer {
	return &Rebalancer{settings: settings}
}

func (r *Rebalancer) RebalanceAsset(
	ctx context.Context,
	asset portfolio.Asset,
	tradeDate time.Time,
) ([]models.SellProposal, []BuyRequest) {
	if asset.Plan == nil {
		log.Print("Evaluating holdings outside the plan for tax losses")
		return r.harvestUnplanned(ctx, asset, tradeDate), nil
	}

	log.Printf("Evaluating %s: at %s, target %s",
		asset.Label(), utils.FormatPercent(asset.Allocation), utils.FormatPercent(asset.Plan.TargetAllocation))
	for _, position := range asset.Positions {
		if position.RecentlySold {
			log.Printf("Skip buying %s: sold within the wash-sale window", position.Ticker)
		}
	}

	sell := r.harvestAsset(ctx, asset, tradeDate)
	shortfall := sell.Proceeds.Sub(asset.ValueDrift)
	if !shortfall.IsPositive() {
		// We should not buy into an overweight asset, so its sale cannot be replaced either.
		log.Printf("Skip %s: not underweight after sales", asset.Label())
		return nil, nil
	}

	buy, ok := selectBuy(asset, sell, shortfall)
	if !ok {
		// This should rarely happen, but if we cannot maintain an asset's allocation
		// by purchasing its paired ticker, we should not harvest a ticker
		log.Printf("Skip %s: no ticker to buy", asset.Label())
		return nil, nil
	}

	log.Printf("Cover a %s shortfall by buying %s", utils.FormatMoney(buy.Amount), buy.Ticker)
	if sell.IsEmpty() {
		return nil, []BuyRequest{buy}
	}
	return []models.SellProposal{sell}, []BuyRequest{buy}
}

// We should not force liquidation of unplanned positions while rebalancing,
// but we should eagerly harvest any tax-loss these positions have accrued.
func (r *Rebalancer) harvestUnplanned(
	ctx context.Context,
	asset portfolio.Asset,
	tradeDate time.Time,
) []models.SellProposal {
	var sells []models.SellProposal
	for _, position := range asset.Positions {
		if sell := r.harvestPosition(ctx, position, tradeDate); !sell.IsEmpty() {
			sells = append(sells, sell)
		}
	}
	return sells
}
