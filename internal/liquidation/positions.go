package liquidation

import (
	"cmp"
	"fmt"
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
	lot         *models.PositionLot
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

func (p *sellablePosition) splitSaleAcrossLots(count int64, saleDate time.Time) []models.LotSale {
	var sales []models.LotSale
	for _, lot := range p.lots {
		if count == 0 {
			break
		}

		sharesFromLot := min(count, lot.wholeShares)
		if sharesFromLot == 0 {
			continue
		}

		sale := models.NewLotSale(p.ticker, lot.lot, p.bid, saleDate)
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
	maxDeviation := *settings.MaxAllocationDeviation

	assets := make([]sellableAsset, len(snapshot.Assets))
	var positions []sellablePosition

	for i, source := range snapshot.Assets {
		asset := &assets[i]
		*asset = newSellableAsset(source, maxDeviation)

		for _, ticker := range source.Tickers {
			heldPosition, isHeld := snapshot.Portfolio.PositionByTicker[ticker]
			if !isHeld || heldPosition.Quantity.IsZero() {
				continue
			}

			quote, quoted := quotes[ticker]
			if !quoted || !quote.Bid.IsPositive() {
				return nil, nil, fmt.Errorf("missing bid price for %s", ticker)
			}

			position, value := newSellablePosition(
				ticker,
				heldPosition,
				quote.Bid,
				asset,
				saleDate,
				rates,
			)
			asset.currentValue = asset.currentValue.Add(value)
			if position.sellableShares > 0 {
				positions = append(positions, position)
			}
		}
	}

	slices.SortStableFunc(positions, comparePositions)

	return positions, assets, nil
}

func newSellablePosition(
	ticker string,
	source models.Position,
	bid decimal.Decimal,
	asset *sellableAsset,
	saleDate time.Time,
	rates *config.TaxSettings,
) (position sellablePosition, currentValue decimal.Decimal) {
	position = sellablePosition{ticker: ticker, asset: asset, bid: bid}
	for i := range source.Lots {
		lot := &source.Lots[i]
		if !lot.Quantity.IsPositive() {
			continue
		}

		sellable, lotValue := newSellableLot(ticker, lot, bid, saleDate, rates)

		position.lots = append(position.lots, sellable)
		position.sellableShares += sellable.wholeShares
		currentValue = currentValue.Add(lotValue)
	}

	slices.SortFunc(position.lots, compareLots)
	return position, currentValue
}

func newSellableLot(
	ticker string,
	lot *models.PositionLot,
	bid decimal.Decimal,
	saleDate time.Time,
	rates *config.TaxSettings,
) (sellable sellableLot, currentValue decimal.Decimal) {
	entireLot := models.NewLotSale(ticker, lot, bid, saleDate)

	sellable = sellableLot{lot: lot, wholeShares: lot.Quantity.Floor().IntPart()}
	if rates != nil {
		sellable.taxPerShare = entireLot.TaxImpactPerShare(*rates)
	}

	return sellable, entireLot.Proceeds()
}

func compareLots(left, right sellableLot) int {
	if order := left.taxPerShare.Cmp(right.taxPerShare); order != 0 {
		return order
	}
	return cmp.Compare(left.lot.Id, right.lot.Id)
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
