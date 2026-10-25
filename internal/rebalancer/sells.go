package rebalancer

import (
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

func (r *Rebalancer) harvestAsset(
	asset portfolio.Asset,
	tradeDate time.Time,
) models.SellProposal {
	if !slices.ContainsFunc(asset.Positions, hasLoss) {
		log.Printf("CANNOT harvest %s: no lots with loss", asset.Label())
		return models.SellProposal{}
	}

	var best models.SellProposal
	for _, position := range asset.Positions {
		if !hasLoss(position) {
			continue
		}
		if !hasReplacement(asset, position.Ticker) {
			log.Printf("CANNOT harvest %s: no replacement ticker to buy", position.Ticker)
			continue
		}
		sell := r.harvestPosition(position, tradeDate)
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

func hasLoss(position portfolio.Position) bool {
	return slices.ContainsFunc(position.Lots, func(lot models.PositionLot) bool {
		return lot.PricePaid.GreaterThan(position.Quote.Bid)
	})
}

func hasReplacement(asset portfolio.Asset, ticker string) bool {
	return slices.ContainsFunc(asset.Positions, func(other portfolio.Position) bool {
		return other.Ticker != ticker && !other.RecentlySold
	})
}

func (r *Rebalancer) harvestPosition(
	position portfolio.Position,
	tradeDate time.Time,
) models.SellProposal {
	ticker := position.Ticker
	if position.RecentlyBought {
		log.Printf("CANNOT harvest %s: bought within wash-sale window", ticker)
		return models.SellProposal{}
	}

	minPctLoss := r.settings.MinPctLossToHarvest
	sales := position.LotSales(tradeDate)
	candidate := models.SellProposal{Ticker: ticker, Price: position.Quote.Bid}
	for _, sale := range sales {
		if isHarvestable(sale, minPctLoss) {
			candidate.Add(sale)
		}
	}
	if candidate.IsEmpty() {
		log.Printf("SKIP harvesting %s: no loss below threshold", ticker)
		return candidate
	}

	log.Printf("CAN harvest %d lot(s) of %s for %s loss", len(candidate.Lots), ticker, utils.FormatMoney(candidate.RealizedPL.Neg()))
	for _, sale := range sales {
		logLot(sale, isHarvestable(sale, minPctLoss))
	}
	return candidate
}

func isHarvestable(sale models.LotSale, minPctLossToHarvest decimal.Decimal) bool {
	gainPct := sale.GainPct()
	return gainPct.IsNegative() && gainPct.LessThanOrEqual(minPctLossToHarvest)
}

func logLot(sale models.LotSale, harvest bool) {
	action := "IGNORE"
	if harvest {
		action = "HARVEST"
	}
	gainPct := sale.GainPct()
	change := utils.FormatPercent(gainPct) + " gain"
	if gainPct.IsNegative() {
		change = utils.FormatPercent(gainPct.Neg()) + " loss"
	}
	log.Printf("  - %s %s share(s) bought %s with %s",
		action, sale.Quantity, sale.Lot.TimeAcquired.Format(time.DateOnly), change)
}
