package liquidation

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type sellableLot struct {
	sale        models.LotSale
	asset       *sellableAsset
	wholeShares int64
	taxPerShare decimal.Decimal
}

func (lot *sellableLot) saleProceeds(shares int64) decimal.Decimal {
	return lot.sale.Price.Mul(decimal.NewFromInt(shares))
}

// Collect every lot with whole shares to sell, in plan order and by lot ID within each position.
func collectLots(
	snapshot portfolio.Snapshot,
	request models.LiquidationRequest,
	amountNeeded decimal.Decimal,
) []sellableLot {
	maxDeviation := request.Settings.AllocationLowerLimit
	rates := request.TaxRates

	totalValue := decimal.Zero
	for _, source := range snapshot.Assets {
		totalValue = totalValue.Add(source.Value)
	}
	totalValueAfterSale := totalValue.Sub(amountNeeded)

	assets := make([]sellableAsset, len(snapshot.Assets))
	var lots []sellableLot

	for i, source := range snapshot.Assets {
		asset := &assets[i]
		*asset = newSellableAsset(source, maxDeviation, totalValueAfterSale)

		for _, sale := range source.LotSales(snapshot.CreatedAt) {
			lot := sellableLot{sale: sale, asset: asset, wholeShares: sale.Quantity.Floor().IntPart()}
			if lot.wholeShares == 0 {
				continue
			}
			if rates != nil {
				lot.taxPerShare = sale.TaxImpactPerShare(*rates)
			}
			lots = append(lots, lot)
		}
	}

	return lots
}

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
