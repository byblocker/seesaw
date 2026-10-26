package liquidation

import (
	"cmp"
	"fmt"
	"slices"

	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func selectLots(lots []sellableLot, minSale decimal.Decimal) ([]models.LotSale, error) {
	// Remaining ties keep plan order, then lot ID order, from collectLots, so plans repeat exactly.
	slices.SortStableFunc(lots, compareLots)
	var sales []models.LotSale
	proceeds := decimal.Zero

	// Sell the most tax-efficient lots first while keeping assets at their minimum allocation.
	for i := range lots {
		lot := &lots[i]
		asset, bid := lot.asset, lot.sale.Price

		amountNeeded := minSale.Sub(proceeds)
		sharesToSell := min(lot.wholeShares, sharesNeeded(amountNeeded, bid))

		if asset.isPlanned {
			valueLeftAboveMin := asset.valueAboveMinAllowed.Sub(asset.sold)
			sharesToSell = min(sharesToSell, sharesNeeded(valueLeftAboveMin, bid))
		}

		if sharesToSell <= 0 {
			continue
		}

		sale := lot.sale
		sale.Quantity = decimal.NewFromInt(sharesToSell)
		sales = append(sales, sale)

		saleProceeds := lot.saleProceeds(sharesToSell)
		proceeds = proceeds.Add(saleProceeds)
		asset.sold = asset.sold.Add(saleProceeds)
	}

	if proceeds.LessThan(minSale) {
		return nil, fmt.Errorf(
			"sales within allocation minimums raise at most $%s of the $%s needed; increase liquidation.max_pct_allocation_deviation",
			proceeds.StringFixed(2), minSale.StringFixed(2),
		)
	}

	return sales, nil
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
