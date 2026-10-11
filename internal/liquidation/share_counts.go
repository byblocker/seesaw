package liquidation

import (
	"context"

	"github.com/shopspring/decimal"
)

// What share counts are allowed for the current position? Must keep proceeds within the funding bounds.
func (search *saleSearch) shareCountRange(depth int, proceedsSoFar decimal.Decimal) (fewest, most int64) {
	position := &search.positions[depth]

	fundingRoom := search.maxSale.Sub(proceedsSoFar)
	most = min(sharesAffordable(fundingRoom, position.bid), position.sellableShares)

	stillNeeded := search.minSale.Sub(proceedsSoFar)
	laterPositionsCanRaise := search.maxProceedsByDepth[depth+1]
	shortfall := stillNeeded.Sub(laterPositionsCanRaise)
	fewest = sharesNeeded(shortfall, position.bid)

	return fewest, most
}

// Picks the share count to try to sell first. The best-case shortfall tends to fall, stays flat at
// its lowest, then rises as share counts increase. Within that flat stretch, the best-case tax has a
// single low point. Find where this stretch starts and ends, and the most shares with the lowest tax within it,
// so ties go to selling more of the earlier, preferred positions.
func (search *saleSearch) bestStartingShares(
	ctx context.Context,
	depth int,
	soldSoFar saleTotals,
	fewest, most int64,
) (int64, error) {
	var err error
	evaluated := map[int64]saleTotals{}
	bestCaseAt := func(shares int64) saleTotals {
		if bestCase, ok := evaluated[shares]; ok {
			return bestCase
		}
		if err == nil {
			err = search.countNode(ctx)
		}
		bestCase := search.bestCase(depth+1, search.withShares(depth, soldSoFar, shares))
		evaluated[shares] = bestCase
		return bestCase
	}

	flatStart := firstShares(fewest, most, func(shares int64) bool {
		return !bestCaseAt(shares + 1).shortfallBelowMin.LessThan(bestCaseAt(shares).shortfallBelowMin)
	})
	leastShortfall := bestCaseAt(flatStart).shortfallBelowMin
	flatEnd := firstShares(flatStart, most, func(shares int64) bool {
		return bestCaseAt(shares + 1).shortfallBelowMin.GreaterThan(leastShortfall)
	})
	leastTax := firstShares(flatStart, flatEnd, func(shares int64) bool {
		return bestCaseAt(shares + 1).tax.GreaterThan(bestCaseAt(shares).tax)
	})

	return leastTax, err
}

// Binary search for the first count in [fewest, most) where holds becomes true, or most.
// Only works when holds is false for every count before that one and true from it onward.
func firstShares(fewest, most int64, holds func(int64) bool) int64 {
	low, high := fewest, most
	for low < high {
		middle := low + (high-low)/2
		if holds(middle) {
			high = middle
		} else {
			low = middle + 1
		}
	}

	return low
}

// Iterate over potential sale share amounts, explore each amount that can beat the best plan,
// and stop as soon as no further amount can improve the plan.
func (search *saleSearch) visitShareCounts(
	ctx context.Context,
	depth int,
	soldSoFar saleTotals,
	first, last, step int64,
) error {
	for shares := first; shares*step <= last*step; shares += step {
		if err := search.countNode(ctx); err != nil {
			return err
		}

		next := search.withShares(depth, soldSoFar, shares)
		bestCase := search.bestCase(depth+1, next)
		if search.canBeatBest(bestCase) {
			if err := search.visit(ctx, depth+1, next); err != nil {
				return err
			}
		} else if search.furtherSharesCannotBeatBest(bestCase) {
			return nil
		}
	}

	return nil
}

// Report whether a losing share count rules out every count further from the starting count.
func (search *saleSearch) furtherSharesCannotBeatBest(bestCase saleTotals) bool {
	shortfall, bestShortfall := bestCase.shortfallBelowMin, search.bestTotals.shortfallBelowMin
	if !shortfall.Equal(bestShortfall) {
		return shortfall.GreaterThan(bestShortfall)
	}

	taxLimit := search.bestTotals.taxOwed().Add(taxRoundingMargin)
	return bestCase.taxOwed().GreaterThan(taxLimit)
}

// Record the share count for the current position and add its sale to the totals.
func (search *saleSearch) withShares(depth int, soldSoFar saleTotals, shares int64) saleTotals {
	position := &search.positions[depth]
	search.chosenSharesByPosition[depth] = shares

	estimatedTax := position.estimateSaleTax(shares)
	estimatedProceeds := position.saleProceeds(shares)

	soldByAsset := search.soldByDepth[depth+1]
	copy(soldByAsset, search.soldByDepth[depth])
	soldByAsset[position.asset.index] = soldByAsset[position.asset.index].Add(estimatedProceeds)

	return soldSoFar.withSale(estimatedProceeds, estimatedTax)
}

func sharesAffordable(amount, price decimal.Decimal) int64 {
	if !amount.IsPositive() {
		return 0
	}

	shares, _ := amount.QuoRem(price, 0)
	return shares.IntPart()
}

func sharesNeeded(amount, price decimal.Decimal) int64 {
	if !amount.IsPositive() {
		return 0
	}

	shares, remainder := amount.QuoRem(price, 0)
	if remainder.IsPositive() {
		return shares.IntPart() + 1
	}

	return shares.IntPart()
}
