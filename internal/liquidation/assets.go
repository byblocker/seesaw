package liquidation

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type sellableAsset struct {
	index                int
	label                string
	isPlanned            bool
	currentValue         decimal.Decimal
	minAllocAllowed      decimal.Decimal
	maxAllocAllowed      decimal.Decimal
	minValueAllowed      decimal.Decimal
	maxValueAllowed      decimal.Decimal
	valueAboveMinAllowed decimal.Decimal
	drift                decimal.Decimal
}

func newSellableAsset(
	index int,
	source portfolio.Asset,
	maxDeviation decimal.Decimal,
	totalValueAfterSale decimal.Decimal,
) sellableAsset {
	asset := sellableAsset{index: index, label: source.Label(), currentValue: source.Value, drift: source.AllocDrift}
	if source.Plan != nil {
		target := source.Plan.TargetAllocation
		asset.isPlanned = true
		asset.minAllocAllowed = utils.ClampNonNegative(target.Sub(maxDeviation))
		asset.maxAllocAllowed = decimal.Min(decimal.NewFromInt(1), target.Add(maxDeviation))
	}

	asset.minValueAllowed = totalValueAfterSale.Mul(asset.minAllocAllowed)
	asset.maxValueAllowed = totalValueAfterSale.Mul(asset.maxAllocAllowed)
	asset.valueAboveMinAllowed = asset.currentValue.Sub(asset.minValueAllowed)
	return asset
}

// Judge against the total the minimums were set from, so warnings match what the selection kept.
func allocationWarnings(
	assets []sellableAsset,
	soldByAsset []decimal.Decimal,
	totalValueAfterSale decimal.Decimal,
) []models.AllocationWarning {
	if !totalValueAfterSale.IsPositive() {
		return nil
	}

	var warnings []models.AllocationWarning
	for i := range assets {
		asset := &assets[i]
		sold := soldByAsset[i]
		if asset.isAllocationAllowed(sold) {
			continue
		}

		valueAfterSale := asset.currentValue.Sub(sold)
		warnings = append(warnings, models.AllocationWarning{
			Asset:           asset.label,
			Allocation:      valueAfterSale.Div(totalValueAfterSale),
			MinAllocAllowed: asset.minAllocAllowed,
			MaxAllocAllowed: asset.maxAllocAllowed,
		})
	}

	return warnings
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
