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
	valueAboveMin []decimal.Decimal,
	amountNeeded decimal.Decimal,
) ([]portfolio.LotSale, error) {
	// Remaining ties keep plan order, then lot ID order, from sellableLots, so plans repeat exactly.
	slices.SortStableFunc(lotIdxs, func(leftLotIdx, rightLotIdx int) int {
		return compareLots(snapshot, leftLotIdx, rightLotIdx)
	})
	var sales []portfolio.LotSale
	proceeds := decimal.Zero
	proceedsByAsset := make([]decimal.Decimal, len(snapshot.Assets))

	// Sell the most tax-efficient lots first while keeping assets at their minimum allocation.
	for _, lotIdx := range lotIdxs {
		lot := snapshot.Lots[lotIdx]
		assetIdx, bid := lot.AssetIdx, lot.Snapshot.Bid

		stillNeeded := amountNeeded.Sub(proceeds)
		if !stillNeeded.IsPositive() {
			break
		}
		sharesToSell := min(lot.WholeShares(), sharesAtLeast(stillNeeded, bid))

		if snapshot.Assets[assetIdx].Plan != nil {
			valueLeftAboveMin := valueAboveMin[assetIdx].Sub(proceedsByAsset[assetIdx])
			sharesToSell = min(sharesToSell, portfolio.SharesAtMost(valueLeftAboveMin, bid))
		}

		if sharesToSell <= 0 {
			continue
		}

		quantity := decimal.NewFromInt(sharesToSell)
		sales = append(sales, portfolio.LotSale{LotIdx: lotIdx, Quantity: quantity})
		saleProceeds := quantity.Mul(bid)
		proceeds = proceeds.Add(saleProceeds)
		proceedsByAsset[assetIdx] = proceedsByAsset[assetIdx].Add(saleProceeds)
	}

	if proceeds.LessThan(amountNeeded) {
		return nil, fmt.Errorf(
			"sales within allocation minimums raise at most $%s of the $%s needed; increase settings.allocation_lower_limit",
			proceeds.StringFixed(2), amountNeeded.StringFixed(2),
		)
	}

	return sales, nil
}

func compareLots(snapshot portfolio.Snapshot, leftLotIdx, rightLotIdx int) int {
	leftLot, rightLot := snapshot.Lots[leftLotIdx], snapshot.Lots[rightLotIdx]
	leftAsset, rightAsset := snapshot.Assets[leftLot.AssetIdx], snapshot.Assets[rightLot.AssetIdx]

	// Sell holdings outside the plan before any planned lot
	if leftIsPlanned, rightIsPlanned := leftAsset.Plan != nil, rightAsset.Plan != nil; leftIsPlanned != rightIsPlanned {
		if leftIsPlanned {
			return 1
		}
		return -1
	}

	return cmp.Or(
		leftLot.Snapshot.TaxPerDollar.Cmp(rightLot.Snapshot.TaxPerDollar),
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
