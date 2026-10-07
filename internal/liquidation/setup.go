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

func prepSearch(
	plan *models.LiquidationPlan,
	snapshot portfolio.Snapshot,
	quotes map[string]models.Quote,
	saleDate time.Time,
) (*saleSearch, []models.SellLot, error) {
	search := &saleSearch{
		minTargetSale: plan.MinTargetSale,
		limit:         plan.Request.SearchLimit,
	}
	search.assets = make([]searchAsset, len(snapshot.Assets))
	var sells []models.SellLot

	for i := range snapshot.Assets {
		assetSells, err := search.collectAssetLots(
			i,
			snapshot.Assets[i].Tickers,
			snapshot.Portfolio,
			quotes,
			saleDate,
		)
		if err != nil {
			return nil, nil, err
		}
		sells = append(sells, assetSells...)
		search.totalValue = search.totalValue.Add(search.assets[i].value)
	}

	search.maxTargetSale = decimal.Min(plan.MaxTargetSale, search.totalValue)

	// Capping proceeds at the collected value keeps this nonnegative.
	remainingValue := search.totalValue.Sub(search.maxTargetSale)
	maxDeviation := *plan.Request.Settings.MaxAllocationDeviation
	for i := range search.assets {
		if err := search.assets[i].setSaleLimit(&snapshot.Assets[i], remainingValue, maxDeviation); err != nil {
			return nil, nil, err
		}
	}

	if plan.Request.Settings.LotSelection == config.EstimatedTax && plan.Request.TaxRates != nil {
		for i := range search.lots {
			search.lots[i].setTaxEstimate(sells[i], *plan.Request.TaxRates)
		}
	}

	// Descending quantities in global preference order resolve equal scores on
	// the first visit, allowing later branches with equal bounds to be pruned.
	order := make([]int, len(search.lots))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(i, j int) int {
		a, b := search.lots[i], search.lots[j]
		aUnplanned := search.assets[a.assetIndex].isUnplanned
		bUnplanned := search.assets[b.assetIndex].isUnplanned
		if aUnplanned != bUnplanned {
			if aUnplanned {
				return -1
			}
			return 1
		}
		return compareLots(sells[i], sells[j], a.taxPerShare, b.taxPerShare, plan.Request.TaxRates != nil)
	})
	// Apply one ordering to both slices so result quantities identify the same lots.
	rankedLots := make([]searchLot, len(order))
	rankedSells := make([]models.SellLot, len(order))
	for i, original := range order {
		rankedLots[i] = search.lots[original]
		rankedSells[i] = sells[original]
	}
	search.lots = rankedLots

	var remaining saleTotals
	for i := len(search.lots) - 1; i >= 0; i-- {
		lot := &search.lots[i]
		lot.accumulateRemainingSales(&remaining, search.assets[lot.assetIndex].isUnplanned)
	}

	search.sold = make([]decimal.Decimal, len(search.assets))
	search.quantities = make([]decimal.Decimal, len(search.lots))
	search.best = make([]decimal.Decimal, len(search.lots))
	return search, rankedSells, nil
}

func (search *saleSearch) collectAssetLots(
	assetIndex int,
	tickers []string,
	holdings *models.Portfolio,
	quotes map[string]models.Quote,
	saleDate time.Time,
) ([]models.SellLot, error) {
	asset := &search.assets[assetIndex]
	var sells []models.SellLot
	for _, ticker := range tickers {
		position, held := holdings.PositionByTicker[ticker]
		if !held || position.Quantity.IsZero() {
			continue
		}

		quote, quoted := quotes[ticker]
		if !quoted || !quote.Bid.IsPositive() {
			return nil, fmt.Errorf("missing bid price for %s", ticker)
		}

		for i := range position.Lots {
			lot := &position.Lots[i]
			if !lot.Quantity.IsPositive() {
				continue
			}

			sell := models.NewSellLot(ticker, lot, quote.Bid, saleDate)
			sells = append(sells, sell)
			search.lots = append(search.lots, searchLot{
				assetIndex: assetIndex, price: sell.Price, available: sell.Quantity.Floor(),
			})
			asset.value = asset.value.Add(sell.Proceeds())
		}
	}
	return sells, nil
}

func (asset *searchAsset) setSaleLimit(source *portfolio.Asset, remainingValue, maxDeviation decimal.Decimal) error {
	one := decimal.NewFromInt(1)
	asset.isUnplanned = source.Plan == nil
	if !asset.isUnplanned {
		target := source.Plan.TargetAllocation
		asset.minWeight = utils.ClampNonNegative(target.Sub(maxDeviation))
		asset.maxWeight = decimal.Min(one, target.Add(maxDeviation))
	}
	_, limit := asset.saleBounds(remainingValue)
	if limit.IsNegative() {
		return fmt.Errorf(
			"%s cannot meet its minimum allocation without purchases", strings.Join(source.Tickers, " / "),
		)
	}
	asset.saleLimit = limit
	return nil
}

func (lot *searchLot) setTaxEstimate(sell models.SellLot, rates config.TaxSettings) {
	lot.taxPerShare = sell.TaxImpactPerShare(rates)
}

func (lot *searchLot) accumulateRemainingSales(remaining *saleTotals, isUnplanned bool) {
	amount := lot.available.Mul(lot.price)
	remaining.proceeds = remaining.proceeds.Add(amount)
	if isUnplanned {
		remaining.unplanned = remaining.unplanned.Add(amount)
	}
	// Taking every remaining loss and no gains is an optimistic tax bound.
	// It stays valid even when allocation or funding prevents those sales.
	if lot.taxPerShare.IsNegative() {
		remaining.tax = remaining.tax.Add(lot.available.Mul(lot.taxPerShare))
	}

	lot.remaining = *remaining
}
