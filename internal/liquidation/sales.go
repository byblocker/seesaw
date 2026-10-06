package liquidation

import (
	"fmt"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func ClampSale(
	sell models.SellLot,
	minProceeds, maxProceeds decimal.Decimal,
	allowFractionalShares bool,
) (models.SellLot, error) {
	if !sell.Price.IsPositive() {
		return models.SellLot{}, fmt.Errorf("sale price must be positive")
	}
	if minProceeds.IsNegative() || maxProceeds.LessThan(minProceeds) {
		return models.SellLot{}, fmt.Errorf("sale proceeds bounds must satisfy 0 <= minimum <= maximum")
	}
	if minProceeds.IsZero() {
		sell.Quantity = decimal.Zero
		return sell, nil
	}

	precision := int32(0)
	if allowFractionalShares {
		precision = 19
	}

	quantity, remainder := minProceeds.QuoRem(sell.Price, precision)
	if remainder.IsPositive() {
		rounded := quantity.Add(decimal.New(1, -precision))
		if rounded.Mul(sell.Price).LessThanOrEqual(maxProceeds) {
			quantity = rounded
		}
	}

	available := sell.Quantity
	if !allowFractionalShares {
		available = available.Floor()
	}

	sell.Quantity = decimal.Min(quantity, available)
	return sell, nil
}
