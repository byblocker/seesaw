package rebalancer

import (
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

func harvestAsset(
	asset portfolio.Asset,
	positions []portfolio.Position,
	lots []models.PositionLot,
	settings config.Settings,
	tradeDate time.Time,
) models.SellProposal {
	if !assetHasLoss(asset, positions, lots) {
		log.Printf("CANNOT harvest %s: no lots with loss", asset.Label)
		return models.SellProposal{}
	}

	var best models.SellProposal
	for _, idx := range asset.PositionIdxs {
		position := positions[idx]
		if !hasLoss(position, lots) {
			continue
		}
		if !hasReplacement(asset, positions, position.Ticker) {
			log.Printf("CANNOT harvest %s: no replacement ticker", position.Ticker)
			continue
		}
		sell := harvestPosition(position, lots, settings, tradeDate)
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

func assetHasLoss(asset portfolio.Asset, positions []portfolio.Position, lots []models.PositionLot) bool {
	return slices.ContainsFunc(asset.PositionIdxs, func(idx int) bool {
		return hasLoss(positions[idx], lots)
	})
}

func hasLoss(position portfolio.Position, lots []models.PositionLot) bool {
	return slices.ContainsFunc(position.LotIdxs, func(idx int) bool {
		return lots[idx].PricePaid.GreaterThan(position.Quote.Bid)
	})
}

func hasReplacement(asset portfolio.Asset, positions []portfolio.Position, ticker string) bool {
	return slices.ContainsFunc(asset.PositionIdxs, func(idx int) bool {
		other := positions[idx]
		return other.Ticker != ticker && !other.RecentlySold
	})
}

func harvestPosition(
	position portfolio.Position,
	lots []models.PositionLot,
	settings config.Settings,
	tradeDate time.Time,
) models.SellProposal {
	ticker := position.Ticker
	if position.RecentlyBought {
		log.Printf("CANNOT harvest %s: bought within wash-sale window", ticker)
		return models.SellProposal{}
	}

	minPctLoss := settings.MinPctLossToHarvest
	sales := position.LotSales(lots, tradeDate)
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
		if sale.GainPct().IsNegative() {
			logLot(sale, isHarvestable(sale, minPctLoss))
		}
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
	log.Printf("  - %s %s share(s) bought %s with %s loss",
		action, sale.Quantity, sale.Lot.TimeAcquired.Format(time.DateOnly), utils.FormatPercent(sale.GainPct().Neg()))
}

// Sell whole shares of an asset worth at most excess, lowest tax per dollar first.
func sellExcess(
	asset portfolio.Asset,
	positions []portfolio.Position,
	lots []models.PositionLot,
	excess decimal.Decimal,
	rates *config.TaxSettings,
	tradeDate time.Time,
) []models.SellProposal {
	sales := portfolio.LotSales(positions, lots, asset.PositionIdxs, tradeDate)
	// Ties keep plan order, then lot ID order, so plans repeat exactly.
	slices.SortStableFunc(sales, func(left, right models.LotSale) int {
		return models.CompareTaxPerDollar(left.TaxImpactPerShare(rates), left.Price, right.TaxImpactPerShare(rates), right.Price)
	})

	var sold []models.LotSale
	remaining := excess
	for _, sale := range sales {
		maxShares, _ := remaining.QuoRem(sale.Price, 0)
		sale.Quantity = decimal.Min(sale.Quantity.Floor(), maxShares)
		if !sale.Quantity.IsPositive() {
			continue
		}
		sold = append(sold, sale)
		remaining = remaining.Sub(sale.Proceeds())
	}

	proposals := models.GroupSales(sold)
	for _, proposal := range proposals {
		proposal.LogProceeds()
	}
	return proposals
}
