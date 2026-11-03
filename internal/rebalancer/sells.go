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
	snapshot portfolio.Snapshot,
	asset portfolio.Asset,
	settings config.Settings,
) models.SellProposal {
	if !assetHasLoss(snapshot, asset) {
		log.Printf("CANNOT harvest %s: no lots with loss", asset.Label)
		return models.SellProposal{}
	}

	var best models.SellProposal
	for _, idx := range asset.PositionIdxs {
		position := snapshot.Positions[idx]
		if !hasLoss(snapshot, position) {
			continue
		}
		if !hasReplacement(snapshot, asset, position.Ticker) {
			log.Printf("CANNOT harvest %s: no replacement ticker", position.Ticker)
			continue
		}
		sell := harvestPosition(snapshot, position, settings)
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

func assetHasLoss(snapshot portfolio.Snapshot, asset portfolio.Asset) bool {
	return slices.ContainsFunc(asset.PositionIdxs, func(idx int) bool {
		return hasLoss(snapshot, snapshot.Positions[idx])
	})
}

func hasLoss(snapshot portfolio.Snapshot, position portfolio.Position) bool {
	return slices.ContainsFunc(position.LotIdxs, func(idx int) bool {
		return snapshot.Lots[idx].PricePaid.GreaterThan(position.Quote.Bid)
	})
}

func hasReplacement(snapshot portfolio.Snapshot, asset portfolio.Asset, ticker string) bool {
	return slices.ContainsFunc(asset.PositionIdxs, func(idx int) bool {
		other := snapshot.Positions[idx]
		return other.Ticker != ticker && !other.RecentlySold
	})
}

func harvestPosition(
	snapshot portfolio.Snapshot,
	position portfolio.Position,
	settings config.Settings,
) models.SellProposal {
	ticker := position.Ticker
	if position.RecentlyBought {
		log.Printf("CANNOT harvest %s: bought within wash-sale window", ticker)
		return models.SellProposal{}
	}

	minPctLoss := settings.MinPctLossToHarvest
	sales := position.LotSales(snapshot)
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
		if sale.GainPct.IsNegative() {
			logLot(snapshot, sale, isHarvestable(sale, minPctLoss))
		}
	}
	return candidate
}

func isHarvestable(sale models.LotSale, minPctLossToHarvest decimal.Decimal) bool {
	gainPct := sale.GainPct
	return gainPct.IsNegative() && gainPct.LessThanOrEqual(minPctLossToHarvest)
}

func logLot(snapshot portfolio.Snapshot, sale models.LotSale, harvest bool) {
	action := "IGNORE"
	if harvest {
		action = "HARVEST"
	}
	log.Printf("  - %s %s share(s) bought %s with %s loss",
		action, sale.Quantity, snapshot.Lots[sale.LotIdx].TimeAcquired.Format(time.DateOnly), utils.FormatPercent(sale.GainPct.Neg()))
}

// Sell whole shares of an asset worth at most excess, lowest tax per dollar first.
func sellExcess(
	snapshot portfolio.Snapshot,
	asset portfolio.Asset,
	excess decimal.Decimal,
	taxPerShare []decimal.Decimal,
) []models.SellProposal {
	var sales []models.LotSale
	for _, idx := range asset.PositionIdxs {
		sales = append(sales, snapshot.Positions[idx].LotSales(snapshot)...)
	}
	// Ties keep plan order, then lot ID order, so plans repeat exactly.
	slices.SortStableFunc(sales, func(left, right models.LotSale) int {
		return models.CompareTaxPerDollar(taxPerShare[left.LotIdx], left.Price, taxPerShare[right.LotIdx], right.Price)
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
