package rebalancer

import (
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"slices"
	"time"
)

type Rebalancer struct {
	settings config.Settings
}

func New(settings config.Settings) *Rebalancer {
	return &Rebalancer{settings: settings}
}

func (r *Rebalancer) RebalanceAsset(
	asset portfolio.Asset,
	tradeDate time.Time,
) ([]models.SellProposal, []BuyRequest) {
	if asset.Plan == nil {
		log.Printf("Rebalancing %s: harvest only", asset.Label())
		return r.harvestUnplanned(asset, tradeDate), nil
	}

	log.Printf("Rebalancing %s: %s --> %s",
		asset.Label(), utils.FormatPercent(asset.Allocation), utils.FormatPercent(asset.Plan.TargetAllocation))
	for _, position := range asset.Positions {
		if position.RecentlySold {
			log.Printf("CANNOT buy %s: sold within wash-sale window", position.Ticker)
		}
	}

	sell := r.harvestAsset(asset, tradeDate)
	shortfall := sell.Proceeds.Sub(asset.ValueDrift)
	if !shortfall.IsPositive() {
		// We should not buy into an overweight asset, so its sale cannot be replaced either.
		log.Printf("CANNOT buy %s: position is overweight", asset.Label())
		return nil, nil
	}

	buy, ok := selectBuy(asset, sell, shortfall)
	if !ok {
		// This should rarely happen, but if we cannot maintain an asset's allocation
		// by purchasing its paired ticker, we should not harvest a ticker
		log.Printf("CANNOT buy %s: all tickers sold within wash-sale window", asset.Label())
		return nil, nil
	}

	if !sell.IsEmpty() {
		sell.LogProceeds()
	}
	log.Printf("Requesting to buy %s of %s . . .", utils.FormatMoney(buy.Amount), buy.Ticker)
	if sell.IsEmpty() {
		return nil, []BuyRequest{buy}
	}
	return []models.SellProposal{sell}, []BuyRequest{buy}
}

// We should not force liquidation of unplanned positions while rebalancing,
// but we should eagerly harvest any tax-loss these positions have accrued.
func (r *Rebalancer) harvestUnplanned(
	asset portfolio.Asset,
	tradeDate time.Time,
) []models.SellProposal {
	if !slices.ContainsFunc(asset.Positions, hasLoss) {
		log.Printf("CANNOT harvest %s: no lots with loss", asset.Label())
		return nil
	}

	var sells []models.SellProposal
	for _, position := range asset.Positions {
		if !hasLoss(position) {
			continue
		}
		if sell := r.harvestPosition(position, tradeDate); !sell.IsEmpty() {
			sell.LogProceeds()
			sells = append(sells, sell)
		}
	}
	return sells
}
