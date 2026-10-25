package liquidation

import (
	"cmp"
	"fmt"
	"slices"

	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

type candidateLot struct {
	position   *sellablePosition
	lot        *sellableLot
	soldShares int64
}

type lotSelection struct {
	sales       []models.LotSale
	proceeds    decimal.Decimal
	soldByAsset []decimal.Decimal
}

func selectLots(positions []sellablePosition, assets []sellableAsset, minSale decimal.Decimal) (lotSelection, error) {
	candidates := cheapestLotsFirst(positions)
	selection := lotSelection{soldByAsset: make([]decimal.Decimal, len(assets))}

	// Sell the most tax-efficient lots first while keeping assets at their minimum allocation.
	for i := range candidates {
		candidate := &candidates[i]
		position, asset := candidate.position, candidate.position.asset

		amountNeeded := minSale.Sub(selection.proceeds)
		remainingShares := candidate.lot.wholeShares - candidate.soldShares

		sharesToSell := min(remainingShares, sharesNeeded(amountNeeded, position.bid))

		if asset.isPlanned {
			valueLeftAboveMin := asset.valueAboveMinAllowed.Sub(selection.soldByAsset[asset.index])
			sharesToSell = min(sharesToSell, sharesNeeded(valueLeftAboveMin, position.bid))
		}

		if sharesToSell <= 0 {
			continue
		}

		candidate.soldShares += sharesToSell
		proceeds := position.saleProceeds(sharesToSell)
		selection.proceeds = selection.proceeds.Add(proceeds)
		selection.soldByAsset[asset.index] = selection.soldByAsset[asset.index].Add(proceeds)
	}

	if selection.proceeds.LessThan(minSale) {
		return lotSelection{}, fmt.Errorf(
			"sales within allocation minimums raise at most $%s of the $%s needed; increase liquidation.max_pct_allocation_deviation",
			selection.proceeds.StringFixed(2), minSale.StringFixed(2),
		)
	}

	for _, candidate := range candidates {
		if candidate.soldShares == 0 {
			continue
		}
		sale := candidate.lot.sale
		sale.Quantity = decimal.NewFromInt(candidate.soldShares)
		selection.sales = append(selection.sales, sale)
	}

	return selection, nil
}

func cheapestLotsFirst(positions []sellablePosition) []candidateLot {
	var candidates []candidateLot
	for i := range positions {
		position := &positions[i]
		for j := range position.lots {
			if position.lots[j].wholeShares > 0 {
				candidates = append(candidates, candidateLot{position: position, lot: &position.lots[j]})
			}
		}
	}

	// Remaining ties keep position and lot order from collectPositions, so plans repeat exactly.
	slices.SortStableFunc(candidates, compareCandidates)
	return candidates
}

func compareCandidates(left, right candidateLot) int {
	// Compare exactly across share prices by scaling tax per share with the bid price
	leftScaled := left.lot.taxPerShare.Mul(right.position.bid)
	rightScaled := right.lot.taxPerShare.Mul(left.position.bid)

	return cmp.Or(
		leftScaled.Cmp(rightScaled),
		// Ties go to the most overweight asset
		right.position.asset.drift.Cmp(left.position.asset.drift),
	)
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
