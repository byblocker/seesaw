package rebalancer

import (
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/portfolio"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

func harvestAsset(
	snapshot portfolio.Snapshot,
	asset portfolio.Asset,
	settings config.Settings,
) portfolio.SellProposal {
	if !assetHasLoss(snapshot, asset) {
		log.Printf("CANNOT harvest %s: no lots with loss", asset.Label)
		return portfolio.SellProposal{}
	}

	var best portfolio.SellProposal
	for _, idx := range asset.PositionIdxs {
		position := snapshot.Positions[idx]
		if !hasLoss(snapshot, position) {
			continue
		}
		if _, ok := replacement(snapshot, asset, idx); !ok {
			log.Printf("CANNOT harvest %s: no replacement ticker", position.Ticker)
			continue
		}
		sell := harvestPosition(snapshot, idx, settings)
		if sell.IsEmpty() {
			continue
		}
		// Keep the deepest loss. Since we begin with the primary position, ties favor selling the alternate.
		if best.IsEmpty() || sell.RealizedPL.LessThanOrEqual(best.RealizedPL) {
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
		return snapshot.Lots[idx].GainPerShare.IsNegative()
	})
}

// The asset's first position other than excludeIdx that can be bought without washing a recent sale.
func replacement(snapshot portfolio.Snapshot, asset portfolio.Asset, excludeIdx int) (int, bool) {
	for _, idx := range asset.PositionIdxs {
		if idx != excludeIdx && !snapshot.Positions[idx].RecentlySold {
			return idx, true
		}
	}
	return -1, false
}

func harvestPosition(
	snapshot portfolio.Snapshot,
	positionIdx int,
	settings config.Settings,
) portfolio.SellProposal {
	position := snapshot.Positions[positionIdx]
	ticker := position.Ticker
	if position.RecentlyBought {
		log.Printf("CANNOT harvest %s: bought within wash-sale window", ticker)
		return portfolio.SellProposal{}
	}

	minPctLoss := settings.MinPctLossToHarvest
	candidate := portfolio.SellProposal{PositionIdx: positionIdx, Price: position.Quote.Bid}
	for _, idx := range position.LotIdxs {
		if lot := snapshot.Lots[idx]; isHarvestable(lot, minPctLoss) {
			candidate.Add(portfolio.LotSale{LotIdx: idx, Quantity: lot.Quantity}, lot)
		}
	}
	if candidate.IsEmpty() {
		log.Printf("SKIP harvesting %s: no loss below threshold", ticker)
		return candidate
	}

	log.Printf("CAN harvest %d lot(s) of %s for %s loss", len(candidate.Lots), ticker, utils.FormatMoney(candidate.RealizedPL.Neg()))
	for _, idx := range position.LotIdxs {
		if lot := snapshot.Lots[idx]; lot.GainPct.IsNegative() {
			logLot(lot, isHarvestable(lot, minPctLoss))
		}
	}
	return candidate
}

func isHarvestable(lot portfolio.LotSnapshot, minPctLossToHarvest decimal.Decimal) bool {
	return lot.GainPct.IsNegative() && lot.GainPct.LessThanOrEqual(minPctLossToHarvest)
}

func logLot(lot portfolio.LotSnapshot, harvest bool) {
	action := "IGNORE"
	if harvest {
		action = "HARVEST"
	}
	log.Printf("  - %s %s share(s) bought %s with %s loss",
		action, lot.Quantity, lot.TimeAcquired.Format(time.DateOnly), utils.FormatPercent(lot.GainPct.Neg()))
}

// Sell whole shares of an asset worth at most excess, lowest tax per dollar first.
func sellExcess(
	snapshot portfolio.Snapshot,
	asset portfolio.Asset,
	excess decimal.Decimal,
	taxPerShare []decimal.Decimal,
) []portfolio.SellProposal {
	var lotIdxs []int
	for _, idx := range asset.PositionIdxs {
		lotIdxs = append(lotIdxs, snapshot.Positions[idx].LotIdxs...)
	}
	// Ties keep plan order, then lot ID order, so plans repeat exactly.
	slices.SortStableFunc(lotIdxs, func(left, right int) int {
		return portfolio.CompareTaxPerDollar(snapshot, taxPerShare, left, right)
	})

	var sold []portfolio.LotSale
	remaining := excess
	for _, lotIdx := range lotIdxs {
		lot := snapshot.Lots[lotIdx]
		bid := snapshot.Positions[lot.PositionIdx].Quote.Bid
		maxShares, _ := remaining.QuoRem(bid, 0)
		quantity := decimal.Min(lot.Quantity.Floor(), maxShares)
		if !quantity.IsPositive() {
			continue
		}
		sold = append(sold, portfolio.LotSale{LotIdx: lotIdx, Quantity: quantity})
		remaining = remaining.Sub(quantity.Mul(bid))
	}

	proposals := portfolio.GroupSales(snapshot, sold)
	for _, proposal := range proposals {
		logProceeds(snapshot, proposal)
	}
	return proposals
}

func logProceeds(snapshot portfolio.Snapshot, sell portfolio.SellProposal) {
	log.Printf("Estimated %s proceeds from selling %s", utils.FormatMoney(sell.Proceeds), snapshot.Positions[sell.PositionIdx].Ticker)
}
