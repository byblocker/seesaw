package liquidation

import (
	"cmp"

	"github.com/shopspring/decimal"
)

type saleTotals struct {
	proceeds decimal.Decimal
	tax      decimal.Decimal
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
		preferLess(totals.taxOwed(), other.taxOwed()),
		preferLess(totals.proceeds, other.proceeds),
	)
}

func preferLess(value, other decimal.Decimal) int {
	return other.Cmp(value)
}
