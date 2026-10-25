package liquidation

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type sellableAsset struct {
	label                string
	isPlanned            bool
	currentValue         decimal.Decimal
	minAllocAllowed      decimal.Decimal
	maxAllocAllowed      decimal.Decimal
	minValueAllowed      decimal.Decimal
	maxValueAllowed      decimal.Decimal
	valueAboveMinAllowed decimal.Decimal
	drift                decimal.Decimal
	sold                 decimal.Decimal
}

func newSellableAsset(
	source portfolio.Asset,
	maxDeviation decimal.Decimal,
	totalValueAfterSale decimal.Decimal,
) sellableAsset {
	asset := sellableAsset{label: source.Label(), currentValue: source.Value, drift: source.AllocDrift}
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
	totalValueAfterSale decimal.Decimal,
) []models.AllocationWarning {
	if !totalValueAfterSale.IsPositive() {
		return nil
	}

	var warnings []models.AllocationWarning
	for i := range assets {
		asset := &assets[i]
		if asset.isAllocationAllowed() {
			continue
		}

		valueAfterSale := asset.currentValue.Sub(asset.sold)
		warnings = append(warnings, models.AllocationWarning{
			Asset:           asset.label,
			Allocation:      valueAfterSale.Div(totalValueAfterSale),
			MinAllocAllowed: asset.minAllocAllowed,
			MaxAllocAllowed: asset.maxAllocAllowed,
		})
	}

	return warnings
}

func (a *sellableAsset) isAllocationAllowed() bool {
	if !a.isPlanned {
		return true
	}

	valueAfterSale := a.currentValue.Sub(a.sold)
	isAboveMin := valueAfterSale.GreaterThanOrEqual(a.minValueAllowed)
	isBelowMax := valueAfterSale.LessThanOrEqual(a.maxValueAllowed)

	return isAboveMin && isBelowMax
}
