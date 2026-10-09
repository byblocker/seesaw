package liquidation

import (
	"cmp"

	"github.com/shopspring/decimal"
)

type saleTotals struct {
	proceeds          decimal.Decimal
	unplannedProceeds decimal.Decimal
	tax               decimal.Decimal
}

func (totals saleTotals) withSale(
	proceeds, tax decimal.Decimal,
	isPlanned bool,
) saleTotals {
	totals.proceeds = totals.proceeds.Add(proceeds)
	totals.tax = totals.tax.Add(tax)

	if !isPlanned {
		totals.unplannedProceeds = totals.unplannedProceeds.Add(proceeds)
	}

	return totals
}

func (totals saleTotals) compare(other saleTotals) int {
	return cmp.Or(
		preferMore(totals.unplannedProceeds, other.unplannedProceeds),
		preferLess(totals.proceeds, other.proceeds),
		preferLess(totals.tax, other.tax),
	)
}

func preferMore(value, other decimal.Decimal) int {
	return value.Cmp(other)
}

func preferLess(value, other decimal.Decimal) int {
	return other.Cmp(value)
}
