package liquidation

import (
	"seesaw/internal/utils"

	"github.com/shopspring/decimal"
)

// Estimates here are optimistic on purpose. No plan reachable from a branch can beat
// its best case, so a branch whose best case loses is skipped without missing a better plan.
// The one exception is proceeds, which carry a small cushion (proceedsTolerance) to prune
// faster. A plan that raises less cash by under that cushion can lose a tie it should win.

// Allow for the rounding taxOnPartOfLot adds when comparing tax estimates.
var taxRoundingMargin = decimal.New(1, -6)

// Estimate the best result still reachable once the first depth positions are decided.
func (search *saleSearch) bestCase(depth int, soldSoFar saleTotals) saleTotals {
	return saleTotals{
		shortfallBelowMin: search.smallestShortfallBelowMin(depth, soldSoFar.proceeds),
		proceeds:          decimal.Max(soldSoFar.proceeds, search.minSale).Add(search.proceedsTolerance),
		tax:               soldSoFar.tax.Add(search.lowestTaxToRaise(depth, soldSoFar.proceeds)),
	}
}

// Explore everything until a first plan exists to compare against.
func (search *saleSearch) canBeatBest(bestCase saleTotals) bool {
	return !search.found || bestCase.compare(search.bestTotals) > 0
}

// Count the shortfall the decided positions already cause, then add what the rest must
// cause. Money still needed beyond their assets' room above minimum falls short dollar for dollar.
func (search *saleSearch) smallestShortfallBelowMin(depth int, proceedsSoFar decimal.Decimal) decimal.Decimal {
	shortfall := decimal.Zero
	roomLeft := decimal.Zero
	for i := range search.assets {
		asset := &search.assets[i]
		sold := search.soldFrom(asset, search.chosenSharesByPosition[:depth])
		unsold := search.unsoldValue(asset, depth)
		shortfall = shortfall.Add(asset.shortfallBelowMin(sold, search.totalValueAfterRequest))

		if !asset.isPlanned {
			roomLeft = roomLeft.Add(unsold)
			continue
		}

		room := utils.ClampNonNegative(asset.roomAboveMin(search.totalValueAfterRequest).Sub(sold))
		roomLeft = roomLeft.Add(decimal.Min(room, unsold))
	}

	stillNeeded := search.minSale.Sub(proceedsSoFar)
	forcedShortfall := utils.ClampNonNegative(stillNeeded.Sub(roomLeft))

	return shortfall.Add(forcedShortfall)
}

// Total what an asset's undecided positions could still sell, which caps how much of its room they can use.
func (search *saleSearch) unsoldValue(asset *sellableAsset, depth int) decimal.Decimal {
	unsold := decimal.Zero
	for i := depth; i < len(search.positions); i++ {
		position := &search.positions[i]
		if position.asset == asset {
			unsold = unsold.Add(position.saleProceeds(position.sellableShares))
		}
	}

	return unsold
}

// Estimate the least tax the undecided positions add while raising what is still needed.
// Take lots cheapest tax per dollar first: losses up to the funding room, then gains only
// as far as needed, splitting the last lot so the estimate stays at or below any real plan.
func (search *saleSearch) lowestTaxToRaise(depth int, proceedsSoFar decimal.Decimal) decimal.Decimal {
	stillNeeded := search.minSale.Sub(proceedsSoFar)
	fundingRoom := search.maxSale.Sub(proceedsSoFar)

	tax := decimal.Zero
	raised := decimal.Zero
	for _, lot := range search.lotsCheapestFirstByDepth[depth] {
		limit := stillNeeded
		if lot.tax.IsNegative() {
			limit = fundingRoom
		}

		available := limit.Sub(raised)
		if !available.IsPositive() {
			break
		}
		if lot.proceeds.GreaterThan(available) {
			return tax.Add(taxOnPartOfLot(lot, available))
		}

		tax = tax.Add(lot.tax)
		raised = raised.Add(lot.proceeds)
	}

	return tax
}

// Round down so the estimate never overstates the lowest possible tax.
func taxOnPartOfLot(lot saleTotals, proceeds decimal.Decimal) decimal.Decimal {
	return lot.tax.Mul(proceeds).DivRound(lot.proceeds, 30).RoundFloor(8)
}
