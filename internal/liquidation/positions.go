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
	plan *models.LiquidationPlan,
) ([]sellablePosition, []sellableAsset) {
	maxDeviation := plan.Request.Settings.MaxAllocationDeviation
	rates := plan.Request.TaxRates

	assets := make([]sellableAsset, len(snapshot.Assets))
	var positions []sellablePosition

	for i, source := range snapshot.Assets {
		asset := &assets[i]
		*asset = newSellableAsset(i, source, maxDeviation)

		for _, position := range source.Positions {
			if sp := newSellablePosition(position, snapshot.CreatedAt, asset, rates); sp.sellableShares > 0 {
				positions = append(positions, sp)
			}
		}
	}

	slices.SortStableFunc(positions, comparePositions)

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
