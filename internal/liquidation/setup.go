package liquidation

import (
	"cmp"
	"fmt"
	"slices"
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
) (*saleSearch, error) {
	search := &saleSearch{
		minTargetSale: plan.MinTargetSale,
		limit:         plan.Request.SearchLimit,
	}
	search.assets = make([]searchAsset, len(snapshot.Assets))

	for i := range snapshot.Assets {
		asset := &search.assets[i]
		asset.source = &snapshot.Assets[i]
		if err := search.collectAssetLots(
			asset,
			snapshot.Portfolio,
			quotes,
			saleDate,
		); err != nil {
			return nil, err
		}

		search.totalValue = search.totalValue.Add(asset.value)
	}

	search.maxTargetSale = decimal.Min(plan.MaxTargetSale, search.totalValue)

	// Capping proceeds at the collected value keeps this nonnegative.
	remainingValue := search.totalValue.Sub(search.maxTargetSale)
	maxDeviation := *plan.Request.Settings.MaxAllocationDeviation
	for i := range search.assets {
		if err := search.assets[i].setSaleLimit(remainingValue, maxDeviation); err != nil {
			return nil, err
		}
	}

	if plan.Request.Settings.LotSelection == config.EstimatedTax && plan.Request.TaxRates != nil {
		for i := range search.lots {
			search.lots[i].setTaxEstimate(*plan.Request.TaxRates)
		}
	}

	// Descending quantities in global preference order resolve equal scores on
	// the first visit, allowing later branches with equal bounds to be pruned.
	slices.SortStableFunc(search.lots, func(a, b searchLot) int {
		if order := cmp.Compare(assetRank(a.asset.source), assetRank(b.asset.source)); order != 0 {
			return order
		}
		return compareLots(a.sell, b.sell, a.taxPerShare, b.taxPerShare, plan.Request.TaxRates != nil)
	})
	var remaining saleTotals
	for i := len(search.lots) - 1; i >= 0; i-- {
		search.lots[i].accumulateRemainingSales(&remaining)
	}

	return search, nil
}

func (search *saleSearch) collectAssetLots(
	asset *searchAsset,
	holdings *models.Portfolio,
	quotes map[string]models.Quote,
	saleDate time.Time,
) error {
	for _, ticker := range asset.source.Tickers {
		position, held := holdings.PositionByTicker[ticker]
		if !held || position.Quantity.IsZero() {
			continue
		}

		quote, quoted := quotes[ticker]
		if !quoted || !quote.Bid.IsPositive() {
			return fmt.Errorf("missing bid price for %s", ticker)
		}

		for i := range position.Lots {
			lot := &position.Lots[i]
			if !lot.Quantity.IsPositive() {
				continue
			}

			sell := models.NewSellLot(ticker, lot, quote.Bid, saleDate)
			search.lots = append(search.lots, searchLot{
				sell: sell, asset: asset, available: sell.Quantity.Floor(),
			})
			asset.value = asset.value.Add(sell.Proceeds())
		}
	}
	return nil
}

func (asset *searchAsset) setSaleLimit(remainingValue, maxDeviation decimal.Decimal) error {
	one := decimal.NewFromInt(1)
	if asset.source.Plan != nil {
		target := asset.source.Plan.TargetAllocation
		asset.minWeight = utils.ClampNonNegative(target.Sub(maxDeviation))
		asset.maxWeight = decimal.Min(one, target.Add(maxDeviation))
	}
	_, limit, err := asset.saleBounds(remainingValue)
	if err != nil {
		return err
	}
	asset.saleLimit = limit
	return nil
}

func (lot *searchLot) setTaxEstimate(rates config.TaxSettings) {
	lot.taxPerShare = lot.sell.TaxImpactPerShare(rates)
}

func (lot *searchLot) accumulateRemainingSales(remaining *saleTotals) {
	amount := lot.available.Mul(lot.sell.Price)
	remaining.proceeds = remaining.proceeds.Add(amount)
	if lot.asset.source.Plan == nil {
		remaining.unplanned = remaining.unplanned.Add(amount)
	}
	// Taking every remaining loss and no gains is an optimistic tax bound.
	// It stays valid even when allocation or funding prevents those sales.
	if lot.taxPerShare.IsNegative() {
		remaining.tax = remaining.tax.Add(lot.available.Mul(lot.taxPerShare))
	}

	lot.remaining = *remaining
}
