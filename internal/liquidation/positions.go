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

// Lots are sorted cheapest first, and a position with no whole shares is never built.
func (p *sellablePosition) cheapestSellableLot() sellableLot {
	for _, lot := range p.lots {
		if lot.wholeShares > 0 {
			return lot
		}
	}
	return p.lots[0]
}

func (p *sellablePosition) saleProceeds(count int64) decimal.Decimal {
	return p.bid.Mul(decimal.NewFromInt(count))
}

func (p *sellablePosition) estimateSaleTax(count int64) decimal.Decimal {
	tax := decimal.Zero
	for _, lot := range p.lots {
		sharesFromLot := min(count, lot.wholeShares)
		tax = tax.Add(lot.taxPerShare.Mul(decimal.NewFromInt(sharesFromLot)))
		count -= sharesFromLot
	}
	return tax
}

func (p *sellablePosition) lotSaleTotals() []saleTotals {
	var totals []saleTotals
	for _, lot := range p.lots {
		if lot.wholeShares == 0 {
			continue
		}

		shares := decimal.NewFromInt(lot.wholeShares)
		totals = append(totals, saleTotals{
			proceeds: p.bid.Mul(shares),
			tax:      lot.taxPerShare.Mul(shares),
		})
	}
	return totals
}

func (p *sellablePosition) splitSaleAcrossLots(count int64) []models.LotSale {
	var sales []models.LotSale
	for _, lot := range p.lots {
		if count == 0 {
			break
		}

		sharesFromLot := min(count, lot.wholeShares)
		if sharesFromLot == 0 {
			continue
		}

		sale := lot.sale
		sale.Quantity = decimal.NewFromInt(sharesFromLot)
		sales = append(sales, sale)
		count -= sharesFromLot
	}
	return sales
}

func collectPositions(
	snapshot portfolio.Snapshot,
	quotes map[string]models.Quote,
	saleDate time.Time,
	settings config.LiquidationSettings,
	rates *config.TaxSettings,
) ([]sellablePosition, []sellableAsset, error) {
	holdingsByAsset, err := snapshot.HoldingsByAsset(quotes, saleDate)
	if err != nil {
		return nil, nil, err
	}

	maxDeviation := settings.MaxAllocationDeviation

	assets := make([]sellableAsset, len(snapshot.Assets))
	var positions []sellablePosition

	for i, holdings := range holdingsByAsset {
		asset := &assets[i]
		*asset = newSellableAsset(i, snapshot.Assets[i], maxDeviation)

		for _, holding := range holdings {
			asset.currentValue = asset.currentValue.Add(holding.Value)
			if position := newSellablePosition(holding, asset, rates); position.sellableShares > 0 {
				positions = append(positions, position)
			}
		}
	}

	slices.SortStableFunc(positions, comparePositions)

	return positions, assets, nil
}

func newSellablePosition(
	holding portfolio.Holding,
	asset *sellableAsset,
	rates *config.TaxSettings,
) sellablePosition {
	position := sellablePosition{ticker: holding.Ticker, asset: asset, bid: holding.Bid}
	for _, sale := range holding.Lots {
		lot := sellableLot{sale: sale, wholeShares: sale.Quantity.Floor().IntPart()}
		if rates != nil {
			lot.taxPerShare = sale.TaxImpactPerShare(*rates)
		}

		position.lots = append(position.lots, lot)
		position.sellableShares += lot.wholeShares
	}

	slices.SortFunc(position.lots, compareLots)
	return position
}

func compareLots(left, right sellableLot) int {
	if order := left.taxPerShare.Cmp(right.taxPerShare); order != 0 {
		return order
	}
	return cmp.Compare(left.sale.Lot.Id, right.sale.Lot.Id)
}

func comparePositions(left, right sellablePosition) int {
	leftAsset, rightAsset := left.asset, right.asset

	// Order unplanned assets before planned ones.
	if leftAsset.isPlanned != rightAsset.isPlanned {
		if leftAsset.isPlanned {
			return 1
		}
		return -1
	}

	// Order by the cheapest sellable lot's tax per dollar incurred
	leftTaxPerDollar := left.cheapestSellableLot().taxPerShare.Mul(right.bid)
	rightTaxPerDollar := right.cheapestSellableLot().taxPerShare.Mul(left.bid)
	if order := leftTaxPerDollar.Cmp(rightTaxPerDollar); order != 0 {
		return order
	}
	return rightAsset.currentValue.Cmp(leftAsset.currentValue)
}
