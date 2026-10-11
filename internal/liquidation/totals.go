package liquidation

import (
	"cmp"

	"github.com/shopspring/decimal"
)

type saleTotals struct {
	shortfallBelowMin decimal.Decimal
	proceeds          decimal.Decimal
	tax               decimal.Decimal
}

func (totals saleTotals) withSale(proceeds, tax decimal.Decimal) saleTotals {
	totals.proceeds = totals.proceeds.Add(proceeds)
	totals.tax = totals.tax.Add(tax)
	return totals
}

func (totals saleTotals) taxOwed() decimal.Decimal {
	// Floor tax at zero to prevent overharvesting tax-losses
	return decimal.Max(totals.tax, decimal.Zero)
}

func (totals saleTotals) compare(other saleTotals) int {
	return cmp.Or(
		// Keep assets at their minimum allocation; when impossible, miss by the fewest dollars.
		preferLess(totals.shortfallBelowMin, other.shortfallBelowMin),
		// Then owe the least tax. Losses beyond zero count for nothing, so extra loss lots are never sold.
		preferLess(totals.taxOwed(), other.taxOwed()),
		// Then raise the amount closest to the request instead of selling more than needed.
		preferLess(totals.proceeds, other.proceeds),
		// Among equally close plans, use up fewer losses, leaving them for future gains.
		preferLess(totals.tax, other.tax),
	)
}

func preferLess(value, other decimal.Decimal) int {
	return other.Cmp(value)
}

func compareTaxPerDollar(left, right saleTotals) int {
	leftTaxPerDollar := left.tax.Mul(right.proceeds)
	rightTaxPerDollar := right.tax.Mul(left.proceeds)
	return leftTaxPerDollar.Cmp(rightTaxPerDollar)
}
