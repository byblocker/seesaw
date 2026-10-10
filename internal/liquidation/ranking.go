package liquidation

import "cmp"

func compareLots(left, right sellableLot) int {
	if order := left.taxPerShare.Cmp(right.taxPerShare); order != 0 {
		return order
	}
	return cmp.Compare(left.lot.Id, right.lot.Id)
}

func compareTaxPerDollar(left, right saleTotals) int {
	leftTaxPerDollar := left.tax.Mul(right.proceeds)
	rightTaxPerDollar := right.tax.Mul(left.proceeds)
	return leftTaxPerDollar.Cmp(rightTaxPerDollar)
}

func comparePositions(left, right sellablePosition) int {
	leftAsset, rightAsset := left.asset, right.asset

	// Order unplanned assets before planned ones.
	if leftAsset.isPlanned != rightAsset.isPlanned {
		if leftAsset.isPlanned {
			return 1
		}
		return -1
	}

	// Order by the cheapest lot's tax per dollar incurred
	leftTaxPerDollar := left.lots[0].taxPerShare.Mul(right.bid)
	rightTaxPerDollar := right.lots[0].taxPerShare.Mul(left.bid)
	if order := leftTaxPerDollar.Cmp(rightTaxPerDollar); order != 0 {
		return order
	}
	return rightAsset.currentValue.Cmp(leftAsset.currentValue)
}
