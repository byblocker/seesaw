package rebalancer

import (
	"context"
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

func (r *Rebalancer) harvestAsset(
	ctx context.Context,
	asset portfolio.Asset,
	tradeDate time.Time,
) models.SellProposal {
	var best models.SellProposal
	for _, position := range asset.Positions {
		if !hasReplacement(asset, position.Ticker) {
			continue
		}
		sell := r.harvestPosition(ctx, position, tradeDate)
		if sell.IsEmpty() {
			continue
		}
		// Since we begin with the primary position, ties favor selling the alternate.
		if best.IsEmpty() || sell.HarvestsAtLeast(best) {
			best = sell
		}
	}
	return best
}

func hasReplacement(asset portfolio.Asset, ticker string) bool {
	return slices.ContainsFunc(asset.Positions, func(other portfolio.Position) bool {
		return other.Ticker != ticker && !other.RecentlySold
	})
}

func (r *Rebalancer) harvestPosition(
	ctx context.Context,
	position portfolio.Position,
	tradeDate time.Time,
) models.SellProposal {
	ticker := position.Ticker
	if !position.IsHeld() {
		return models.SellProposal{}
	}
	if position.RecentlyBought {
		log.Printf("Skip harvesting %s: bought within the wash-sale window", ticker)
		return models.SellProposal{}
	}

	minPctLoss := r.settings.MinPctLossToHarvest
	candidate := harvestLots(position, tradeDate, minPctLoss)
	if candidate.IsEmpty() {
		log.Printf("Skip harvesting %s: no lots down %s or more",
			ticker, utils.FormatPercent(minPctLoss.Neg()))
	} else {
		log.Printf("Can harvest %d lot(s) of %s for %s at a %s loss",
			len(candidate.Lots), ticker, utils.FormatMoney(candidate.Proceeds), utils.FormatMoney(candidate.RealizedPL.Neg()))
	}
	return candidate
}

func harvestLots(position portfolio.Position, saleDate time.Time, minPctLossToHarvest decimal.Decimal) models.SellProposal {
	proposal := models.SellProposal{
		Ticker: position.Ticker,
		Price:  position.Quote.Bid,
	}

	for _, sale := range position.LotSales(saleDate) {
		gainPct := sale.GainPct()
		if gainPct.IsNegative() && gainPct.LessThanOrEqual(minPctLossToHarvest) {
			proposal.Add(sale)
		}
	}

	return proposal
}
