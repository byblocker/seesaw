package liquidation

import (
	"seesaw/pkg/models"
	"time"

	"github.com/shopspring/decimal"
)

func EstimateTax(sale models.SellProposal, rates models.TaxSettings, saleDate time.Time) decimal.Decimal {
	var tax decimal.Decimal
	for _, sell := range sale.Lots {
		gain := sell.RealizedPL(sale.Price)
		if !gain.IsPositive() {
			continue
		}

		rate := rates.ShortTermGainRate
		if sell.Lot.IsLongTermAt(saleDate) {
			rate = rates.LongTermGainRate
		}

		tax = tax.Add(gain.Mul(*rate))
	}

	return tax
}
