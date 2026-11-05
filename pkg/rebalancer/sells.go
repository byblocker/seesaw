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
	for _, positionIdx := range asset.PositionIdxs {
		position := snapshot.Positions[positionIdx]
		if !hasLoss(snapshot, position) {
			continue
		}
		if _, ok := replacement(snapshot, asset, positionIdx); !ok {
			log.Printf("CANNOT harvest %s: no replacement ticker", position.Ticker)
			continue
		}
		sell := harvestPosition(snapshot, positionIdx, settings)
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
	return slices.ContainsFunc(asset.PositionIdxs, func(positionIdx int) bool {
		return hasLoss(snapshot, snapshot.Positions[positionIdx])
	})
}

func hasLoss(snapshot portfolio.Snapshot, position portfolio.Position) bool {
	return slices.ContainsFunc(position.LotIdxs, func(lotIdx int) bool {
		return snapshot.Lots[lotIdx].Snapshot.GainPerShare.IsNegative()
	})
}

// The asset's first position other than excludedPositionIdx that can be bought without washing a recent sale.
func replacement(snapshot portfolio.Snapshot, asset portfolio.Asset, excludedPositionIdx int) (int, bool) {
	for _, positionIdx := range asset.PositionIdxs {
		if positionIdx != excludedPositionIdx && !snapshot.Positions[positionIdx].RecentlySold {
			return positionIdx, true
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
	candidate := portfolio.SellProposal{PositionIdx: positionIdx}
	for _, lotIdx := range position.LotIdxs {
		if lot := snapshot.Lots[lotIdx]; isHarvestable(lot, minPctLoss) {
			candidate.Add(portfolio.LotSale{LotIdx: lotIdx, Quantity: lot.Quantity}, lot)
		}
	}
	if candidate.IsEmpty() {
		log.Printf("SKIP harvesting %s: no loss below threshold", ticker)
		return candidate
	}

	log.Printf("CAN harvest %d lot(s) of %s for %s loss", len(candidate.Lots), ticker, utils.FormatMoney(candidate.RealizedPL.Neg()))
	for _, lotIdx := range position.LotIdxs {
		if lot := snapshot.Lots[lotIdx]; lot.Snapshot.GainPct.IsNegative() {
			logLot(lot, isHarvestable(lot, minPctLoss))
		}
	}
	return candidate
}

func isHarvestable(lot portfolio.Lot, minPctLossToHarvest decimal.Decimal) bool {
	return lot.Snapshot.GainPct.IsNegative() && lot.Snapshot.GainPct.LessThanOrEqual(minPctLossToHarvest)
}

func logLot(lot portfolio.Lot, harvest bool) {
	action := "IGNORE"
	if harvest {
		action = "HARVEST"
	}
	log.Printf("  - %s %s share(s) bought %s with %s loss",
		action, lot.Quantity, lot.TimeAcquired.Format(time.DateOnly), utils.FormatPercent(lot.Snapshot.GainPct.Neg()))
}

// Sell whole shares of an asset worth at most excess, lowest tax per dollar first.
func sellExcess(
	snapshot portfolio.Snapshot,
	asset portfolio.Asset,
	excess decimal.Decimal,
	taxPerDollar []decimal.Decimal,
) []portfolio.SellProposal {
	var lotIdxs []int
	for _, positionIdx := range asset.PositionIdxs {
		lotIdxs = append(lotIdxs, snapshot.Positions[positionIdx].LotIdxs...)
	}
	// Ties keep plan order, then lot ID order, so plans repeat exactly.
	slices.SortStableFunc(lotIdxs, func(leftLotIdx, rightLotIdx int) int {
		return taxPerDollar[leftLotIdx].Cmp(taxPerDollar[rightLotIdx])
	})

	var sold []portfolio.LotSale
	remaining := excess
	for _, lotIdx := range lotIdxs {
		lot := snapshot.Lots[lotIdx]
		bid := lot.Snapshot.Bid
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
