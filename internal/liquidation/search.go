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
	totalValueAfterRequest   decimal.Decimal
	proceedsTolerance        decimal.Decimal
	maxProceedsByDepth       []decimal.Decimal
	lotsCheapestFirstByDepth [][]saleTotals
	nodeLimit                int
	visitedNodes             int

	// Indexed by depth, then asset: what positions from depth onward could still sell.
	unsoldByDepth [][]decimal.Decimal
	// Indexed by depth, then asset: what the positions before depth sell. withShares fills row depth+1.
	soldByDepth [][]decimal.Decimal

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
		unsoldByDepth:            make([][]decimal.Decimal, len(positions)+1),
		soldByDepth:              make([][]decimal.Decimal, len(positions)+1),
		chosenSharesByPosition:   make([]int64, len(positions)),
	}
	totalValue := decimal.Zero
	for _, asset := range assets {
		totalValue = totalValue.Add(asset.currentValue)
	}

	search.maxSale = decimal.Min(plan.MaxTargetSale, totalValue)
	search.totalValueAfterRequest = totalValue.Sub(search.minSale)

	for depth := range search.soldByDepth {
		search.soldByDepth[depth] = make([]decimal.Decimal, len(assets))
	}
	search.unsoldByDepth[len(positions)] = make([]decimal.Decimal, len(assets))

	for i := len(positions) - 1; i >= 0; i-- {
		position := &positions[i]
		positionProceeds := position.saleProceeds(position.sellableShares)
		search.maxProceedsByDepth[i] = search.maxProceedsByDepth[i+1].Add(positionProceeds)

		unsold := slices.Clone(search.unsoldByDepth[i+1])
		unsold[position.asset.index] = unsold[position.asset.index].Add(positionProceeds)
		search.unsoldByDepth[i] = unsold

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

func (search *saleSearch) recordIfBest(sold saleTotals) {
	sold.shortfallBelowMin = decimal.Zero
	for i, soldFromAsset := range search.soldByDepth[len(search.positions)] {
		sold.shortfallBelowMin = sold.shortfallBelowMin.Add(search.assets[i].shortfallBelowMin(soldFromAsset))
	}
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
	// Judge against the same total the search used, so warnings match what it optimized.
	totalValueAfterSale := search.totalValueAfterRequest
	if !totalValueAfterSale.IsPositive() {
		return nil
	}

	var warnings []models.AllocationWarning
	for i := range search.assets {
		asset := &search.assets[i]
		sold := search.soldFrom(asset, search.bestSharesByPosition)
		if asset.isAllocationAllowed(sold) {
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
