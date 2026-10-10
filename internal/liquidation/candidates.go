package liquidation

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"seesaw/internal/utils"
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

type sellableAsset struct {
	label           string
	isPlanned       bool
	currentValue    decimal.Decimal
	minAllocAllowed decimal.Decimal
	maxAllocAllowed decimal.Decimal
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
