package liquidation

import (
	"cmp"

	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

// compareLots orders lots by estimated tax per dollar raised, then by ticker and lot.
func compareLots(a, b models.SellLot, aTax, bTax decimal.Decimal) int {
	// Quantity cancels out of tax per dollar; cross multiplication avoids rounding.
	if order := aTax.Mul(b.Price).Cmp(bTax.Mul(a.Price)); order != 0 {
		return order
	}
	if order := cmp.Compare(a.Ticker, b.Ticker); order != 0 {
		return order
	}
	return cmp.Compare(a.Lot.Id, b.Lot.Id)
}
