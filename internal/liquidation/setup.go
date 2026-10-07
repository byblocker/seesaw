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

func prepareSaleSearch(
	snapshot portfolio.Snapshot,
	quotes map[string]models.Quote,
	saleDate time.Time,
	plan *models.LiquidationPlan,
) (*saleSearch, error) {
	request := plan.Request
	search := &saleSearch{
		minTargetSale: plan.MinTargetSale,
		limit:         request.SearchLimit,
	}
	search.assets = make([]searchAsset, len(snapshot.Assets))
	for i := range snapshot.Assets {
		asset := &search.assets[i]
		asset.source = &snapshot.Assets[i]

		for _, ticker := range asset.source.Tickers {
			position, held := snapshot.Portfolio.PositionByTicker[ticker]
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
				search.lots = append(search.lots, searchLot{
					sell: sell, asset: asset, available: sell.Quantity.Floor(),
				})
				asset.value = asset.value.Add(sell.Proceeds())
			}
		}
		search.totalValue = search.totalValue.Add(asset.value)
	}

	search.maxTargetSale = decimal.Min(plan.MaxTargetSale, search.totalValue)
	// Capping proceeds at the collected value keeps this nonnegative.
	remainingValue := search.totalValue.Sub(search.maxTargetSale)
	one := decimal.NewFromInt(1)
	maxDeviation := *request.Settings.MaxAllocationDeviation
	for i := range search.assets {
		asset := &search.assets[i]
		if asset.source.Plan != nil {
			target := asset.source.Plan.TargetAllocation
			asset.minWeight = utils.ClampNonNegative(target.Sub(maxDeviation))
			asset.maxWeight = decimal.Min(one, target.Add(maxDeviation))
		}
		_, limit, err := asset.saleBounds(remainingValue)
		if err != nil {
			return nil, err
		}
		asset.saleLimit = limit
	}
	if request.Settings.LotSelection == config.EstimatedTax && request.TaxRates != nil {
		for i := range search.lots {
			lot := &search.lots[i]
			lot.taxPerShare = lot.sell.TaxImpactPerShare(*request.TaxRates)
		}
	}
	// Descending quantities in global preference order resolve equal scores on
	// the first visit, allowing later branches with equal bounds to be pruned.
	slices.SortStableFunc(search.lots, func(a, b searchLot) int {
		if order := cmp.Compare(assetRank(a.asset.source), assetRank(b.asset.source)); order != 0 {
			return order
		}
		return compareLots(a.sell, b.sell, a.taxPerShare, b.taxPerShare, request.TaxRates != nil)
	})
	remaining := saleTotals{}
	for i := len(search.lots) - 1; i >= 0; i-- {
		lot := &search.lots[i]
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

		lot.remaining = remaining
	}

	return search, nil
}
