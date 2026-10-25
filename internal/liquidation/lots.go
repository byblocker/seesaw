package liquidation

import (
	"cmp"
	"slices"

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

// Collect every lot with whole shares to sell, in plan order and by lot ID within each position,
// and the portfolio value the allocation minimums are measured against.
func collectLots(
	snapshot portfolio.Snapshot,
	plan *models.LiquidationPlan,
) ([]sellableLot, []sellableAsset, decimal.Decimal) {
	maxDeviation := plan.Request.Settings.MaxAllocationDeviation
	rates := plan.Request.TaxRates

	totalValue := decimal.Zero
	for _, source := range snapshot.Assets {
		totalValue = totalValue.Add(source.Value)
	}
	totalValueAfterSale := totalValue.Sub(plan.MinTargetSale)

	assets := make([]sellableAsset, len(snapshot.Assets))
	var lots []sellableLot

	for i, source := range snapshot.Assets {
		asset := &assets[i]
		*asset = newSellableAsset(source, maxDeviation, totalValueAfterSale)

		for _, position := range source.Positions {
			sales := position.LotSales(snapshot.CreatedAt)
			slices.SortFunc(sales, func(left, right models.LotSale) int {
				return cmp.Compare(left.Lot.Id, right.Lot.Id)
			})

			for _, sale := range sales {
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
	}

	return lots, assets, totalValueAfterSale
}
