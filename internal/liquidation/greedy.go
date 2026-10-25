package liquidation

import (
	"cmp"
	"fmt"
	"slices"

	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

type lotSelection struct {
	sales       []models.LotSale
	proceeds    decimal.Decimal
	soldByAsset []decimal.Decimal
}

func selectLots(lots []sellableLot, assets []sellableAsset, minSale decimal.Decimal) (lotSelection, error) {
	// Remaining ties keep plan order, then lot ID order, from collectLots, so plans repeat exactly.
	slices.SortStableFunc(lots, compareLots)
	selection := lotSelection{soldByAsset: make([]decimal.Decimal, len(assets))}

	// Sell the most tax-efficient lots first while keeping assets at their minimum allocation.
	for i := range lots {
		lot := &lots[i]
		asset, bid := lot.asset, lot.sale.Price

		amountNeeded := minSale.Sub(selection.proceeds)
		sharesToSell := min(lot.wholeShares, sharesNeeded(amountNeeded, bid))

		if asset.isPlanned {
			valueLeftAboveMin := asset.valueAboveMinAllowed.Sub(selection.soldByAsset[asset.index])
			sharesToSell = min(sharesToSell, sharesNeeded(valueLeftAboveMin, bid))
		}

		if sharesToSell <= 0 {
			continue
		}

		sale := lot.sale
		sale.Quantity = decimal.NewFromInt(sharesToSell)
		selection.sales = append(selection.sales, sale)

		proceeds := lot.saleProceeds(sharesToSell)
		selection.proceeds = selection.proceeds.Add(proceeds)
		selection.soldByAsset[asset.index] = selection.soldByAsset[asset.index].Add(proceeds)
	}

	if selection.proceeds.LessThan(minSale) {
		return lotSelection{}, fmt.Errorf(
			"sales within allocation minimums raise at most $%s of the $%s needed; increase liquidation.max_pct_allocation_deviation",
			selection.proceeds.StringFixed(2), minSale.StringFixed(2),
		)
	}

	return selection, nil
}

func compareLots(left, right sellableLot) int {
	// Sell holdings outside the plan before any planned lot
	if left.asset.isPlanned != right.asset.isPlanned {
		if left.asset.isPlanned {
			return 1
		}
		return -1
	}

	// Compare exactly across share prices by scaling tax per share with the bid price
	leftScaled := left.taxPerShare.Mul(right.sale.Price)
	rightScaled := right.taxPerShare.Mul(left.sale.Price)

	return cmp.Or(
		leftScaled.Cmp(rightScaled),
		// Ties go to the most overweight asset
		right.asset.drift.Cmp(left.asset.drift),
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
