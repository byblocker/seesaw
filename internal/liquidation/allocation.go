package liquidation

import (
	"fmt"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"strings"

	"github.com/shopspring/decimal"
)

type allocationWeights struct {
	lower decimal.Decimal
	upper decimal.Decimal
}

func prepareAllocationWeights(asset *models.Asset, remainingInvestedValue, maxDeviation decimal.Decimal) (allocationWeights, error) {
	one := decimal.NewFromInt(1)
	if remainingInvestedValue.IsNegative() {
		return allocationWeights{}, fmt.Errorf("remaining invested value must be nonnegative")
	}
	if maxDeviation.IsNegative() || maxDeviation.GreaterThan(one) {
		return allocationWeights{}, fmt.Errorf("allocation deviation must be between 0 and 1")
	}
	if asset.Plan == nil {
		return allocationWeights{}, nil
	}
	return allocationWeights{
		lower: utils.ClampNonNegative(asset.Plan.TargetAllocation.Sub(maxDeviation)),
		upper: decimal.Min(one, asset.Plan.TargetAllocation.Add(maxDeviation)),
	}, nil
}

func (weights allocationWeights) saleBounds(asset *models.Asset, value, remainingInvestedValue decimal.Decimal) (minSale, maxSale decimal.Decimal, err error) {
	if asset.Plan == nil {
		return decimal.Zero, value, nil
	}
	minRemaining := remainingInvestedValue.Mul(weights.lower)
	maxRemaining := remainingInvestedValue.Mul(weights.upper)

	if value.LessThan(minRemaining) {
		return decimal.Zero, decimal.Zero, fmt.Errorf(
			"%s cannot meet its minimum allocation without purchases", strings.Join(asset.Tickers, " / "),
		)
	}

	minSale = utils.ClampNonNegative(value.Sub(maxRemaining))
	maxSale = value.Sub(minRemaining)

	return minSale, maxSale, nil
}
