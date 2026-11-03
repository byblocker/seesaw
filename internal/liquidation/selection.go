package liquidation

import (
	"cmp"
	"fmt"
	"slices"

	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

func selectLots(
	snapshot portfolio.Snapshot,
	lotIdxs []int,
	taxPerShare []decimal.Decimal,
	valueAboveMin []decimal.Decimal,
	amountNeeded decimal.Decimal,
) ([]portfolio.LotSale, error) {
	// Remaining ties keep plan order, then lot ID order, from sellableLots, so plans repeat exactly.
	slices.SortStableFunc(lotIdxs, func(left, right int) int {
		return compareLots(snapshot, taxPerShare, left, right)
	})
	var sales []portfolio.LotSale
	proceeds := decimal.Zero
	sold := make([]decimal.Decimal, len(snapshot.Assets))

	// Sell the most tax-efficient lots first while keeping assets at their minimum allocation.
	for _, lotIdx := range lotIdxs {
		lot := snapshot.Lots[lotIdx]
		position := snapshot.Positions[lot.PositionIdx]
		assetIdx, bid := position.AssetIdx, position.Quote.Bid

		stillNeeded := amountNeeded.Sub(proceeds)
		sharesToSell := min(wholeShares(lot), sharesAtLeast(stillNeeded, bid))

		if snapshot.Assets[assetIdx].Plan != nil {
			valueLeftAboveMin := valueAboveMin[assetIdx].Sub(sold[assetIdx])
			sharesToSell = min(sharesToSell, sharesAtMost(valueLeftAboveMin, bid))
		}

		if sharesToSell <= 0 {
			continue
		}

		sale := snapshot.LotSale(lotIdx, decimal.NewFromInt(sharesToSell))
		sales = append(sales, sale)
		proceeds = proceeds.Add(sale.Proceeds())
		sold[assetIdx] = sold[assetIdx].Add(sale.Proceeds())
	}

	if proceeds.LessThan(amountNeeded) {
		return nil, fmt.Errorf(
			"sales within allocation minimums raise at most $%s of the $%s needed; increase settings.allocation_lower_limit",
			proceeds.StringFixed(2), amountNeeded.StringFixed(2),
		)
	}

	return sales, nil
}

func compareLots(snapshot portfolio.Snapshot, taxPerShare []decimal.Decimal, left, right int) int {
	leftAsset := snapshot.Assets[snapshot.Positions[snapshot.Lots[left].PositionIdx].AssetIdx]
	rightAsset := snapshot.Assets[snapshot.Positions[snapshot.Lots[right].PositionIdx].AssetIdx]

	// Sell holdings outside the plan before any planned lot
	if leftIsPlanned, rightIsPlanned := leftAsset.Plan != nil, rightAsset.Plan != nil; leftIsPlanned != rightIsPlanned {
		if leftIsPlanned {
			return 1
		}
		return -1
	}

	return cmp.Or(
		portfolio.CompareTaxPerDollar(snapshot, taxPerShare, left, right),
		// Ties go to the most overweight asset
		rightAsset.AllocDrift.Cmp(leftAsset.AllocDrift),
	)
}

// Whole shares at price that raise at least amount.
func sharesAtLeast(amount, price decimal.Decimal) int64 {
	if !amount.IsPositive() {
		return 0
	}

	shares, remainder := amount.QuoRem(price, 0)
	if remainder.IsPositive() {
		return shares.IntPart() + 1
	}

	return shares.IntPart()
}

// Whole shares at price that raise at most amount.
func sharesAtMost(amount, price decimal.Decimal) int64 {
	if !amount.IsPositive() {
		return 0
	}

	shares, _ := amount.QuoRem(price, 0)
	return shares.IntPart()
}
