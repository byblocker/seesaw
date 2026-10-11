package liquidation

import (
	"context"
	"fmt"
	"slices"

	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

// How close to the request a branch's proceeds must be to be explored.
var pctProceedsTolerance = decimal.NewFromFloat(0.001)

// Decide how many shares to sell of each position, one position at a time. For each
// position, try the most promising share amounts first and work outwards. Skip when
// current best possible cannot beat the best plan so far. The first plan found wins ties.
type saleSearch struct {
	positions                []sellablePosition
	assets                   []sellableAsset
	minSale                  decimal.Decimal
	maxSale                  decimal.Decimal
	totalValue               decimal.Decimal
	totalValueAfterRequest   decimal.Decimal
	proceedsTolerance        decimal.Decimal
	maxProceedsByDepth       []decimal.Decimal
	lotsCheapestFirstByDepth [][]saleTotals
	nodeLimit                int
	visitedNodes             int

	chosenSharesByPosition []int64
	bestSharesByPosition   []int64
	bestTotals             saleTotals
	found                  bool
}

func newSaleSearch(
	positions []sellablePosition,
	assets []sellableAsset,
	plan *models.LiquidationPlan,
) (*saleSearch, error) {
	search := &saleSearch{
		positions:                positions,
		assets:                   assets,
		minSale:                  plan.MinTargetSale,
		proceedsTolerance:        plan.Request.Amount.Mul(pctProceedsTolerance),
		nodeLimit:                plan.Request.SearchLimit,
		maxProceedsByDepth:       make([]decimal.Decimal, len(positions)+1),
		lotsCheapestFirstByDepth: make([][]saleTotals, len(positions)+1),
		chosenSharesByPosition:   make([]int64, len(positions)),
	}
	for _, asset := range assets {
		search.totalValue = search.totalValue.Add(asset.currentValue)
	}

	search.maxSale = decimal.Min(plan.MaxTargetSale, search.totalValue)
	search.totalValueAfterRequest = search.totalValue.Sub(search.minSale)

	for i := len(positions) - 1; i >= 0; i-- {
		position := &positions[i]
		positionProceeds := position.saleProceeds(position.sellableShares)
		search.maxProceedsByDepth[i] = search.maxProceedsByDepth[i+1].Add(positionProceeds)

		lots := append(slices.Clone(search.lotsCheapestFirstByDepth[i+1]), position.lotSaleTotals()...)
		slices.SortStableFunc(lots, compareTaxPerDollar)
		search.lotsCheapestFirstByDepth[i] = lots
	}

	sellableValue := search.maxProceedsByDepth[0]
	if sellableValue.LessThan(search.minSale) {
		return nil, fmt.Errorf(
			"whole-share holdings can raise at most $%s of the $%s needed",
			sellableValue.StringFixed(2), search.minSale.StringFixed(2),
		)
	}

	return search, nil
}

func (search *saleSearch) run(ctx context.Context) error {
	return search.visit(ctx, 0, saleTotals{})
}

// Visit the next position pending potential sales, and decide how many shares to sell.
func (search *saleSearch) visit(
	ctx context.Context,
	depth int,
	soldSoFar saleTotals,
) error {
	if depth == len(search.positions) {
		search.recordIfBest(soldSoFar)
		return nil
	}

	fewest, most := search.shareCountRange(depth, soldSoFar.proceeds)
	if fewest > most {
		return nil
	}

	start, err := search.bestStartingShares(ctx, depth, soldSoFar, fewest, most)
	if err != nil {
		return err
	}

	if err := search.visitShareCounts(ctx, depth, soldSoFar, start, fewest, -1); err != nil {
		return err
	}
	return search.visitShareCounts(ctx, depth, soldSoFar, start+1, most, 1)
}

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
// single low point. Find where this stretch starts and ends, and record the lowest tax owed within it.
func (search *saleSearch) bestStartingShares(
	ctx context.Context,
	depth int,
	soldSoFar saleTotals,
	fewest, most int64,
) (int64, error) {
	var err error
	bestCaseAt := func(shares int64) saleTotals {
		if err == nil {
			err = search.countNode(ctx)
		}
		return search.bestCase(depth+1, search.withShares(depth, soldSoFar, shares))
	}

	flatStart := firstShares(fewest, most, func(shares int64) bool {
		return !bestCaseAt(shares + 1).shortfallBelowMin.LessThan(bestCaseAt(shares).shortfallBelowMin)
	})
	leastShortfall := bestCaseAt(flatStart).shortfallBelowMin
	flatEnd := firstShares(flatStart, most, func(shares int64) bool {
		return bestCaseAt(shares + 1).shortfallBelowMin.GreaterThan(leastShortfall)
	})
	leastTax := firstShares(flatStart, flatEnd, func(shares int64) bool {
		return !bestCaseAt(shares + 1).tax.LessThan(bestCaseAt(shares).tax)
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
	return soldSoFar.withSale(estimatedProceeds, estimatedTax)
}

func (search *saleSearch) recordIfBest(sold saleTotals) {
	sold.shortfallBelowMin = search.shortfallBelowMin(search.chosenSharesByPosition)
	if search.found && sold.compare(search.bestTotals) <= 0 {
		return
	}

	search.found = true
	search.bestTotals = sold
	search.bestSharesByPosition = append(search.bestSharesByPosition[:0], search.chosenSharesByPosition...)
}

func (search *saleSearch) countNode(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	search.visitedNodes++
	if search.visitedNodes > search.nodeLimit {
		return fmt.Errorf("liquidation search limit reached; increase the limit or narrow the request")
	}

	return nil
}

func (search *saleSearch) shortfallBelowMin(sharesByPosition []int64) decimal.Decimal {
	shortfall := decimal.Zero
	for i := range search.assets {
		asset := &search.assets[i]
		sold := search.soldFrom(asset, sharesByPosition)
		shortfall = shortfall.Add(asset.shortfallBelowMin(sold, search.totalValueAfterRequest))
	}

	return shortfall
}

func (search *saleSearch) soldFrom(asset *sellableAsset, sharesByPosition []int64) decimal.Decimal {
	sold := decimal.Zero
	for i, shares := range sharesByPosition {
		position := &search.positions[i]
		if position.asset == asset {
			sold = sold.Add(position.saleProceeds(shares))
		}
	}

	return sold
}

func (search *saleSearch) allocationWarnings() []models.AllocationWarning {
	totalValueAfterSale := search.totalValue.Sub(search.bestTotals.proceeds)
	if !totalValueAfterSale.IsPositive() {
		return nil
	}

	var warnings []models.AllocationWarning
	for i := range search.assets {
		asset := &search.assets[i]
		sold := search.soldFrom(asset, search.bestSharesByPosition)
		if asset.isAllocationAllowed(sold, totalValueAfterSale) {
			continue
		}

		valueAfterSale := asset.currentValue.Sub(sold)
		warnings = append(warnings, models.AllocationWarning{
			Asset:           asset.label,
			Allocation:      valueAfterSale.Div(totalValueAfterSale),
			MinAllocAllowed: asset.minAllocAllowed,
			MaxAllocAllowed: asset.maxAllocAllowed,
		})
	}

	return warnings
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
