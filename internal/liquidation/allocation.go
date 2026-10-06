package liquidation

import (
	"fmt"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"strings"

	"github.com/shopspring/decimal"
)

func AllocationSaleBounds(
	asset models.Asset,
	remainingInvestedValue, maxDeviation decimal.Decimal,
) (minSale, maxSale decimal.Decimal, err error) {
	one := decimal.NewFromInt(1)
	if remainingInvestedValue.IsNegative() {
		return decimal.Zero, decimal.Zero, fmt.Errorf("remaining invested value must be nonnegative")
	}
	if maxDeviation.IsNegative() || maxDeviation.GreaterThan(one) {
		return decimal.Zero, decimal.Zero, fmt.Errorf("allocation deviation must be between 0 and 1")
	}
	if asset.Plan == nil {
		return decimal.Zero, asset.Value, nil
	}

	lowerWeight := utils.ClampNonNegative(asset.Plan.TargetAllocation.Sub(maxDeviation))
	minRemaining := remainingInvestedValue.Mul(lowerWeight)

	upperWeight := decimal.Min(one, asset.Plan.TargetAllocation.Add(maxDeviation))
	maxRemaining := remainingInvestedValue.Mul(upperWeight)

	if asset.Value.LessThan(minRemaining) {
		return decimal.Zero, decimal.Zero, fmt.Errorf(
			"%s cannot meet its minimum allocation without purchases", strings.Join(asset.Tickers, " / "),
		)
	}

	minSale = utils.ClampNonNegative(asset.Value.Sub(maxRemaining))
	maxSale = asset.Value.Sub(minRemaining)

	return minSale, maxSale, nil
}
