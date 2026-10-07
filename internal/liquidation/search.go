package liquidation

import (
	"cmp"
	"context"
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

type searchAsset struct {
	source    *portfolio.Asset
	value     decimal.Decimal
	minWeight decimal.Decimal
	maxWeight decimal.Decimal
	saleLimit decimal.Decimal
	sold      decimal.Decimal
}

func (asset *searchAsset) saleBounds(remainingInvestedValue decimal.Decimal) (minSale, maxSale decimal.Decimal, err error) {
	if asset.source.Plan == nil {
		return decimal.Zero, asset.value, nil
	}
	minRemaining := remainingInvestedValue.Mul(asset.minWeight)
	maxRemaining := remainingInvestedValue.Mul(asset.maxWeight)

	if asset.value.LessThan(minRemaining) {
		return decimal.Zero, decimal.Zero, fmt.Errorf(
			"%s cannot meet its minimum allocation without purchases", strings.Join(asset.source.Tickers, " / "),
		)
	}

	minSale = utils.ClampNonNegative(asset.value.Sub(maxRemaining))
	maxSale = asset.value.Sub(minRemaining)

	return minSale, maxSale, nil
}

type searchLot struct {
	sell        models.SellLot
	asset       *searchAsset
	available   decimal.Decimal
	quantity    decimal.Decimal
	best        decimal.Decimal
	taxPerShare decimal.Decimal
	remaining   saleTotals
}

type saleTotals struct {
	proceeds  decimal.Decimal
	unplanned decimal.Decimal
	tax       decimal.Decimal
}

type saleSearch struct {
	assets        []searchAsset
	lots          []searchLot
	minTargetSale decimal.Decimal
	maxTargetSale decimal.Decimal
	totalValue    decimal.Decimal
	found         bool
	bestTotals    saleTotals
	nodes         int
	limit         int
}

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
	if err := search.collectLots(snapshot, quotes, saleDate); err != nil {
		return nil, err
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

func (search *saleSearch) collectLots(
	snapshot portfolio.Snapshot,
	quotes map[string]models.Quote,
	saleDate time.Time,
) error {
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
		search.totalValue = search.totalValue.Add(asset.value)
	}

	return nil
}

func (search *saleSearch) visit(ctx context.Context, lotIndex int, totals saleTotals) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	search.nodes++
	if search.nodes > search.limit {
		return fmt.Errorf("liquidation search limit reached; increase the limit or narrow the request")
	}
	if !search.canImprove(lotIndex, totals) {
		return nil
	}
	if lotIndex == len(search.lots) {
		search.consider(totals)
		return nil
	}

	lot := &search.lots[lotIndex]
	asset, sell := lot.asset, lot.sell
	minQuantity, maxQuantity := search.quantityBounds(lotIndex, totals.proceeds)
	one := decimal.NewFromInt(1)
	for quantity := maxQuantity; quantity.GreaterThanOrEqual(minQuantity); quantity = quantity.Sub(one) {
		amount := quantity.Mul(sell.Price)
		lot.quantity = quantity
		previousSold := asset.sold
		asset.sold = previousSold.Add(amount)
		next := totals
		next.proceeds = next.proceeds.Add(amount)
		next.tax = next.tax.Add(quantity.Mul(lot.taxPerShare))
		if asset.source.Plan == nil {
			next.unplanned = next.unplanned.Add(amount)
		}
		err := search.visit(ctx, lotIndex+1, next)
		asset.sold = previousSold
		if err != nil {
			return err
		}
	}
	return nil
}

func (search *saleSearch) canImprove(lotIndex int, totals saleTotals) bool {
	remaining := search.remainingAt(lotIndex)
	if totals.proceeds.Add(remaining.proceeds).LessThan(search.minTargetSale) {
		return false
	}
	if !search.found {
		return true
	}
	possible := saleTotals{
		tax:      totals.tax.Add(remaining.tax),
		proceeds: decimal.Max(totals.proceeds, search.minTargetSale),
		unplanned: totals.unplanned.Add(decimal.Min(
			remaining.unplanned, search.maxTargetSale.Sub(totals.proceeds),
		)),
	}
	return compareSaleTotals(possible, search.bestTotals) > 0
}

func (search *saleSearch) consider(totals saleTotals) {
	order := compareSaleTotals(totals, search.bestTotals)
	if search.found && order <= 0 {
		return
	}
	remainingValue := search.totalValue.Sub(totals.proceeds)
	for _, asset := range search.assets {
		minSale, maxSale, err := asset.saleBounds(remainingValue)
		if err != nil || asset.sold.LessThan(minSale) || asset.sold.GreaterThan(maxSale) {
			return
		}
	}
	for i := range search.lots {
		lot := &search.lots[i]
		lot.best = lot.quantity
	}
	search.found = true
	search.bestTotals = totals
}

// compareSaleTotals prefers more unplanned proceeds, fewer total proceeds, then lower tax.
func compareSaleTotals(a, b saleTotals) int {
	if order := a.unplanned.Cmp(b.unplanned); order != 0 {
		return order
	}
	if order := b.proceeds.Cmp(a.proceeds); order != 0 {
		return order
	}
	return b.tax.Cmp(a.tax)
}

func (search *saleSearch) remainingAt(lotIndex int) saleTotals {
	if lotIndex == len(search.lots) {
		return saleTotals{}
	}
	return search.lots[lotIndex].remaining
}

func (search *saleSearch) quantityBounds(lotIndex int, proceeds decimal.Decimal) (minQuantity, maxQuantity decimal.Decimal) {
	lot := &search.lots[lotIndex]
	asset, sell := lot.asset, lot.sell
	capacity := decimal.Min(search.maxTargetSale.Sub(proceeds), asset.saleLimit.Sub(asset.sold))
	maxQuantity, _ = capacity.QuoRem(sell.Price, 0)
	maxQuantity = decimal.Min(maxQuantity, lot.available)
	needed := search.minTargetSale.Sub(proceeds).Sub(search.remainingAt(lotIndex + 1).proceeds)
	minQuantity, remainder := decimal.Max(decimal.Zero, needed).QuoRem(sell.Price, 0)
	if remainder.IsPositive() {
		minQuantity = minQuantity.Add(decimal.NewFromInt(1))
	}
	return minQuantity, maxQuantity
}
