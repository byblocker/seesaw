package liquidation

import (
	"cmp"
	"slices"
	"time"

	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type sellablePosition struct {
	ticker         string
	asset          *sellableAsset
	bid            decimal.Decimal
	lots           []sellableLot
	sellableShares int64
}

type sellableLot struct {
	sale        models.LotSale // the whole lot
	wholeShares int64
	taxPerShare decimal.Decimal
}

func (p *sellablePosition) saleProceeds(count int64) decimal.Decimal {
	return p.bid.Mul(decimal.NewFromInt(count))
}

func collectPositions(
	snapshot portfolio.Snapshot,
	plan *models.LiquidationPlan,
) ([]sellablePosition, []sellableAsset) {
	maxDeviation := plan.Request.Settings.MaxAllocationDeviation
	rates := plan.Request.TaxRates

	totalValue := decimal.Zero
	for _, source := range snapshot.Assets {
		totalValue = totalValue.Add(source.Value)
	}
	totalValueAfterSale := totalValue.Sub(plan.MinTargetSale)

	assets := make([]sellableAsset, len(snapshot.Assets))
	var positions []sellablePosition

	for i, source := range snapshot.Assets {
		asset := &assets[i]
		*asset = newSellableAsset(i, source, maxDeviation, totalValueAfterSale)

		for _, position := range source.Positions {
			if sp := newSellablePosition(position, snapshot.CreatedAt, asset, rates); sp.sellableShares > 0 {
				positions = append(positions, sp)
			}
		}
	}

	return positions, assets
}

func newSellablePosition(
	held portfolio.Position,
	saleDate time.Time,
	asset *sellableAsset,
	rates *config.TaxSettings,
) sellablePosition {
	position := sellablePosition{ticker: held.Ticker, asset: asset, bid: held.Quote.Bid}
	for _, sale := range held.LotSales(saleDate) {
		lot := sellableLot{sale: sale, wholeShares: sale.Quantity.Floor().IntPart()}
		if rates != nil {
			lot.taxPerShare = sale.TaxImpactPerShare(*rates)
		}

		position.lots = append(position.lots, lot)
		position.sellableShares += lot.wholeShares
	}

	slices.SortFunc(position.lots, func(left, right sellableLot) int {
		return cmp.Compare(left.sale.Lot.Id, right.sale.Lot.Id)
	})
	return position
}
