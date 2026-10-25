package liquidation

import (
	"seesaw/internal/utils"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type sellableAsset struct {
	isPlanned            bool
	valueAboveMinAllowed decimal.Decimal
	drift                decimal.Decimal
	sold                 decimal.Decimal
}

func newSellableAsset(
	source portfolio.Asset,
	maxDeviation decimal.Decimal,
	totalValueAfterSale decimal.Decimal,
) sellableAsset {
	asset := sellableAsset{drift: source.AllocDrift}
	minAllocAllowed := decimal.Zero
	if source.Plan != nil {
		asset.isPlanned = true
		minAllocAllowed = utils.ClampNonNegative(source.Plan.TargetAllocation.Sub(maxDeviation))
	}

	asset.valueAboveMinAllowed = source.Value.Sub(totalValueAfterSale.Mul(minAllocAllowed))
	return asset
}
