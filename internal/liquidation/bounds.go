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
	soldByAsset, unsoldByAsset := search.soldByDepth[depth], search.unsoldByDepth[depth]

	shortfall := decimal.Zero
	roomLeft := decimal.Zero
	for i := range search.assets {
		asset := &search.assets[i]
		sold, unsold := soldByAsset[i], unsoldByAsset[i]
		shortfall = shortfall.Add(asset.shortfallBelowMin(sold))

		if !asset.isPlanned {
			roomLeft = roomLeft.Add(unsold)
			continue
		}

		room := utils.ClampNonNegative(asset.valueAboveMinAllowed.Sub(sold))
		roomLeft = roomLeft.Add(decimal.Min(room, unsold))
	}

	stillNeeded := search.minSale.Sub(proceedsSoFar)
	forcedShortfall := utils.ClampNonNegative(stillNeeded.Sub(roomLeft))

	return shortfall.Add(forcedShortfall)
}

// Estimate the least tax the undecided positions add while raising what is still needed.
// Take lots cheapest tax per dollar first: losses up to the funding room, then gains only
// as far as needed, splitting the last lot so the estimate stays at or below any real plan.
//
// Once the best plan keeps every asset at its minimum, a plan can only beat it by doing the
// same, so each planned asset gives up no more than its room above minimum. With one cap per
// asset and one overall limit, cheapest first still finds the least tax, so the estimate holds.
func (search *saleSearch) lowestTaxToRaise(depth int, proceedsSoFar decimal.Decimal) decimal.Decimal {
	stillNeeded := search.minSale.Sub(proceedsSoFar)
	fundingRoom := search.maxSale.Sub(proceedsSoFar)

	capRoom := search.found && search.bestTotals.shortfallBelowMin.IsZero()
	room := search.roomAboveMinLeft
	if capRoom {
		for i := range search.assets {
			room[i] = utils.ClampNonNegative(search.assets[i].valueAboveMinAllowed.Sub(search.soldByDepth[depth][i]))
		}
	}

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

		proceeds := lot.proceeds
		if capRoom && search.assets[lot.asset].isPlanned {
			proceeds = decimal.Min(proceeds, room[lot.asset])
			room[lot.asset] = room[lot.asset].Sub(proceeds)
		}
		if !proceeds.IsPositive() {
			continue
		}
		if proceeds.GreaterThan(available) {
			return tax.Add(taxOnPartOfLot(lot, available))
		}

		if proceeds.Equal(lot.proceeds) {
			tax = tax.Add(lot.tax)
		} else {
			tax = tax.Add(taxOnPartOfLot(lot, proceeds))
		}
		raised = raised.Add(proceeds)
	}

	return tax
}

// Round down so the estimate never overstates the lowest possible tax.
func taxOnPartOfLot(lot lotTotals, proceeds decimal.Decimal) decimal.Decimal {
	return lot.tax.Mul(proceeds).DivRound(lot.proceeds, 30).RoundFloor(8)
}
