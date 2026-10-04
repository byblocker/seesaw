package liquidation

import (
	"seesaw/pkg/models"
	"time"

	"github.com/shopspring/decimal"
)

type TaxEstimator struct {
	shortTermGainRate decimal.Decimal
	longTermGainRate  decimal.Decimal
	saleDate          time.Time
}

func NewTaxEstimator(
	rates models.TaxSettings,
	saleDate time.Time,
) (TaxEstimator, error) {
	if err := rates.Validate(); err != nil {
		return TaxEstimator{}, err
	}

	return TaxEstimator{
		shortTermGainRate: *rates.ShortTermGainRate,
		longTermGainRate:  *rates.LongTermGainRate,
		saleDate:          saleDate,
	}, nil
}

// Estimate applies gain rates per lot; it does not assume a benefit from losses.
func (estimator TaxEstimator) Estimate(sale models.SellProposal) (decimal.Decimal, error) {
	if err := sale.Validate(); err != nil {
		return decimal.Zero, err
	}

	var tax decimal.Decimal
	for _, sell := range sale.Lots {
		gain := sell.RealizedPL(sale.Price)
		if !gain.IsPositive() {
			continue
		}

		rate := estimator.shortTermGainRate
		if sell.Lot.IsLongTermAt(estimator.saleDate) {
			rate = estimator.longTermGainRate
		}
		tax = tax.Add(gain.Mul(rate))
	}
	return tax, nil
}
