package liquidation

import (
	"cmp"

	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func compareLots(
	a, b models.SellLot,
	aTax, bTax decimal.Decimal,
	taxable bool,
) int {
	if taxable {
		if !aTax.IsZero() || !bTax.IsZero() {
			// Quantity cancels out of tax per dollar; cross multiplication avoids rounding.
			if order := aTax.Mul(b.Price).Cmp(bTax.Mul(a.Price)); order != 0 {
				return order
			}
		}
		if order := cmp.Compare(taxCategory(a.GainPerShare, a.LongTerm), taxCategory(b.GainPerShare, b.LongTerm)); order != 0 {
			return order
		}
		if order := a.GainPerShare.Mul(b.Price).Cmp(b.GainPerShare.Mul(a.Price)); order != 0 {
			return order
		}
	}

	if order := cmp.Compare(a.Ticker, b.Ticker); order != 0 {
		return order
	}

	return cmp.Compare(a.Lot.Id, b.Lot.Id)
}

const (
	shortTermLossRank = iota
	longTermLossRank
	breakEvenRank
	longTermGainRank
	shortTermGainRank
)

func taxCategory(gain decimal.Decimal, longTerm bool) int {
	sign := gain.Sign()
	switch {
	case sign < 0 && !longTerm:
		return shortTermLossRank
	case sign < 0:
		return longTermLossRank
	case sign == 0:
		return breakEvenRank
	case longTerm:
		return longTermGainRank
	default:
		return shortTermGainRank
	}
}
