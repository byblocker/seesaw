package liquidation

import (
	"strings"

	"seesaw/internal/utils"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type sellableAsset struct {
	label           string
	isPlanned       bool
	currentValue    decimal.Decimal
	minAllocAllowed decimal.Decimal
	maxAllocAllowed decimal.Decimal
}

func newSellableAsset(source portfolio.Asset, maxDeviation decimal.Decimal) sellableAsset {
	asset := sellableAsset{label: strings.Join(source.Tickers, " / ")}
	if source.Plan == nil {
		return asset
	}

	target := source.Plan.TargetAllocation
	asset.isPlanned = true
	asset.minAllocAllowed = utils.ClampNonNegative(target.Sub(maxDeviation))
	asset.maxAllocAllowed = decimal.Min(decimal.NewFromInt(1), target.Add(maxDeviation))
	return asset
}

func (a *sellableAsset) shortfallBelowMin(sold, totalValueAfterSale decimal.Decimal) decimal.Decimal {
	if !a.isPlanned {
		return decimal.Zero
	}

	return utils.ClampNonNegative(sold.Sub(a.roomAboveMin(totalValueAfterSale)))
}

func (a *sellableAsset) roomAboveMin(totalValueAfterSale decimal.Decimal) decimal.Decimal {
	minValueAllowed := totalValueAfterSale.Mul(a.minAllocAllowed)
	return a.currentValue.Sub(minValueAllowed)
}

func (a *sellableAsset) isAllocationAllowed(sold, totalValueAfterSale decimal.Decimal) bool {
	if !a.isPlanned {
		return true
	}

	valueAfterSale := a.currentValue.Sub(sold)
	minValueAllowed := totalValueAfterSale.Mul(a.minAllocAllowed)
	maxValueAllowed := totalValueAfterSale.Mul(a.maxAllocAllowed)

	isAboveMin := valueAfterSale.GreaterThanOrEqual(minValueAllowed)
	isBelowMax := valueAfterSale.LessThanOrEqual(maxValueAllowed)

	return isAboveMin && isBelowMax
}
