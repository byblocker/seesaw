package liquidation

import (
	"seesaw/internal/utils"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type sellableAsset struct {
	index           int
	label           string
	isPlanned       bool
	currentValue    decimal.Decimal
	minAllocAllowed decimal.Decimal
	maxAllocAllowed decimal.Decimal

	// Fixed by setValueBounds once the portfolio total after the sale is known.
	minValueAllowed decimal.Decimal
	maxValueAllowed decimal.Decimal
	roomAboveMin    decimal.Decimal
}

func newSellableAsset(index int, source portfolio.Asset, maxDeviation decimal.Decimal) sellableAsset {
	asset := sellableAsset{index: index, label: source.Label()}
	if source.Plan == nil {
		return asset
	}

	target := source.Plan.TargetAllocation
	asset.isPlanned = true
	asset.minAllocAllowed = utils.ClampNonNegative(target.Sub(maxDeviation))
	asset.maxAllocAllowed = decimal.Min(decimal.NewFromInt(1), target.Add(maxDeviation))
	return asset
}

func (a *sellableAsset) setValueBounds(totalValueAfterSale decimal.Decimal) {
	a.minValueAllowed = totalValueAfterSale.Mul(a.minAllocAllowed)
	a.maxValueAllowed = totalValueAfterSale.Mul(a.maxAllocAllowed)
	a.roomAboveMin = a.currentValue.Sub(a.minValueAllowed)
}

func (a *sellableAsset) shortfallBelowMin(sold decimal.Decimal) decimal.Decimal {
	if !a.isPlanned {
		return decimal.Zero
	}

	return utils.ClampNonNegative(sold.Sub(a.roomAboveMin))
}

func (a *sellableAsset) isAllocationAllowed(sold decimal.Decimal) bool {
	if !a.isPlanned {
		return true
	}

	valueAfterSale := a.currentValue.Sub(sold)
	isAboveMin := valueAfterSale.GreaterThanOrEqual(a.minValueAllowed)
	isBelowMax := valueAfterSale.LessThanOrEqual(a.maxValueAllowed)

	return isAboveMin && isBelowMax
}
