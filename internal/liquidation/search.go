package liquidation

import (
	"context"
	"fmt"

	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

// Search depth-first branch and bound over how many shares to sell of each
// position, which are visited in preference order with share counts descending.
// The first plan found wins ties and later plans must be strictly better.
type saleSearch struct {
	positions                []sellablePosition
	assets                   []sellableAsset
	minSale                  decimal.Decimal
	maxSale                  decimal.Decimal
	totalValue               decimal.Decimal
	totalValueAfterRequest   decimal.Decimal
	bestCaseRemainingByDepth []saleTotals
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
		nodeLimit:                plan.Request.SearchLimit,
		bestCaseRemainingByDepth: make([]saleTotals, len(positions)+1),
		chosenSharesByPosition:   make([]int64, len(positions)),
	}
	for _, asset := range assets {
		search.totalValue = search.totalValue.Add(asset.currentValue)
	}

	search.maxSale = decimal.Min(plan.MaxTargetSale, search.totalValue)
	search.totalValueAfterRequest = search.totalValue.Sub(search.minSale)

	for i := len(positions) - 1; i >= 0; i-- {
		position := &positions[i]
		remaining := search.bestCaseRemainingByDepth[i+1]

		estimatedTax := position.estimateTaxSellingOnlyLosses()
		estimatedProceeds := position.saleProceeds(position.sellableShares)
		search.bestCaseRemainingByDepth[i] = remaining.withSale(estimatedProceeds, estimatedTax)
	}

	sellableValue := search.bestCaseRemainingByDepth[0].proceeds
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

func (search *saleSearch) visit(ctx context.Context, depth int, soldSoFar saleTotals) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	search.visitedNodes++
	if search.visitedNodes > search.nodeLimit {
		return fmt.Errorf("liquidation search limit reached; increase the limit or narrow the request")
	}
	if !search.canBeatBest(depth, soldSoFar) {
		return nil
	}
	if depth == len(search.positions) {
		search.recordIfBest(soldSoFar)
		return nil
	}

	position := &search.positions[depth]
	fewest, most := search.shareCountRange(depth, soldSoFar.proceeds)
	for shares := most; shares >= fewest; shares-- {
		search.chosenSharesByPosition[depth] = shares

		estimatedTax := position.estimateSaleTax(shares)
		estimatedProceeds := position.saleProceeds(shares)
		next := soldSoFar.withSale(estimatedProceeds, estimatedTax)
		if err := search.visit(ctx, depth+1, next); err != nil {
			return err
		}
	}
	return nil
}

func (search *saleSearch) canBeatBest(depth int, soldSoFar saleTotals) bool {
	bestCaseRemaining := search.bestCaseRemainingByDepth[depth]
	if soldSoFar.proceeds.Add(bestCaseRemaining.proceeds).LessThan(search.minSale) {
		return false
	}
	if !search.found {
		return true
	}
	bestCaseTotals := saleTotals{
		shortfallBelowMin: search.shortfallBelowMin(search.chosenSharesByPosition[:depth]),
		proceeds:          decimal.Max(soldSoFar.proceeds, search.minSale),
		tax:               soldSoFar.tax.Add(bestCaseRemaining.tax),
	}
	return bestCaseTotals.compare(search.bestTotals) > 0
}

func (search *saleSearch) shareCountRange(depth int, proceedsSoFar decimal.Decimal) (fewest, most int64) {
	position := &search.positions[depth]

	fundingRoom := search.maxSale.Sub(proceedsSoFar)
	most = min(sharesAffordable(fundingRoom, position.bid), position.sellableShares)

	stillNeeded := search.minSale.Sub(proceedsSoFar)
	laterPositionsCanRaise := search.bestCaseRemainingByDepth[depth+1].proceeds
	shortfall := stillNeeded.Sub(laterPositionsCanRaise)
	fewest = sharesNeeded(shortfall, position.bid)

	return fewest, most
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

func (search *saleSearch) shortfallBelowMin(sharesByPosition []int64) decimal.Decimal {
	shortfall := decimal.Zero
	for i := range search.assets {
		asset := &search.assets[i]
		sold := search.soldFrom(asset, sharesByPosition)
		shortfall = shortfall.Add(asset.shortfallBelowMin(sold, search.totalValueAfterRequest))
	}

	return shortfall
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
