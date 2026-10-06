package liquidation

import (
	"cmp"
	"context"
	"fmt"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/orders"
	"slices"

	"github.com/shopspring/decimal"
)

type searchAsset struct {
	candidate *LiquidationCandidate
	weights   allocationWeights
	saleLimit decimal.Decimal
	sold      decimal.Decimal
}

type searchLot struct {
	sell        *orders.SellLot
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
	assets     []searchAsset
	lots       []searchLot
	funding    Funding
	totalValue decimal.Decimal
	found      bool
	bestTotals saleTotals
	nodes      int
	limit      int
}

func SelectWholeShareSales(
	ctx context.Context,
	groups []LiquidationCandidate,
	funding Funding,
	request models.LiquidationRequest,
) ([]orders.SellProposal, error) {
	if funding.MinTargetSale.IsNegative() || funding.MaxTargetSale.LessThan(funding.MinTargetSale) {
		return nil, fmt.Errorf("invalid sale funding bounds")
	}
	if funding.MinTargetSale.IsZero() {
		return nil, nil
	}
	if request.SearchLimit <= 0 {
		return nil, fmt.Errorf("search limit must be positive")
	}
	if request.Settings.LotSelection != config.EstimatedTax && request.Settings.LotSelection != config.MinTaxOrder {
		return nil, fmt.Errorf("unsupported lot selection %q", request.Settings.LotSelection)
	}
	if request.TaxRates != nil && request.Settings.LotSelection == config.EstimatedTax {
		if err := request.TaxRates.Validate(); err != nil {
			return nil, err
		}
	}

	search, err := prepareSaleSearch(groups, funding, request)
	if err != nil {
		return nil, err
	}
	search.limit = request.SearchLimit
	if err := searchSales(ctx, search, 0, saleTotals{}); err != nil {
		return nil, err
	}
	if !search.found {
		return nil, fmt.Errorf("no whole-share liquidation meets the funding and allocation limits")
	}
	return assembleSales(search.lots), nil
}

func prepareSaleSearch(
	groups []LiquidationCandidate,
	funding Funding,
	request models.LiquidationRequest,
) (*saleSearch, error) {
	search := &saleSearch{
		funding: funding,
		assets:  make([]searchAsset, len(groups)),
	}
	lotCount := 0
	for i := range groups {
		search.assets[i].candidate = &groups[i]
		search.totalValue = search.totalValue.Add(groups[i].Value)
		lotCount += len(groups[i].LotsToSell)
	}

	search.funding.MaxTargetSale = decimal.Min(funding.MaxTargetSale, search.totalValue)
	search.lots = make([]searchLot, 0, lotCount)
	remainingValue := search.totalValue.Sub(search.funding.MaxTargetSale)
	for i := range search.assets {
		asset := &search.assets[i]
		weights, err := prepareAllocationWeights(asset.candidate.Asset, remainingValue, *request.Settings.MaxAllocationDeviation)
		if err != nil {
			return nil, err
		}
		asset.weights = weights
		_, limit, err := selectionBounds(*asset, remainingValue)
		if err != nil {
			return nil, err
		}
		asset.saleLimit = limit
		for j := range asset.candidate.LotsToSell {
			sell := &asset.candidate.LotsToSell[j]
			lot := searchLot{sell: sell, asset: asset, available: sell.Quantity.Floor()}
			if request.Settings.LotSelection == config.EstimatedTax && request.TaxRates != nil {
				lot.taxPerShare = sell.TaxImpactPerShare(*request.TaxRates)
			}
			search.lots = append(search.lots, lot)
		}
	}
	// Descending quantities in global preference order resolve equal scores on
	// the first visit, allowing later branches with equal bounds to be pruned.
	slices.SortStableFunc(search.lots, func(a, b searchLot) int {
		if order := cmp.Compare(assetRank(a.asset.candidate.Asset), assetRank(b.asset.candidate.Asset)); order != 0 {
			return order
		}
		return compareLots(*a.sell, *b.sell, a.taxPerShare, b.taxPerShare, request.TaxRates != nil)
	})
	remaining := saleTotals{}
	for i := len(search.lots) - 1; i >= 0; i-- {
		lot := &search.lots[i]
		amount := lot.available.Mul(lot.sell.Price)
		remaining.proceeds = remaining.proceeds.Add(amount)
		if lot.asset.candidate.Asset.Plan == nil {
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

func searchSales(ctx context.Context, search *saleSearch, lotIndex int, totals saleTotals) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	search.nodes++
	if search.nodes > search.limit {
		return fmt.Errorf("liquidation search limit reached; increase the limit or narrow the request")
	}
	if !canImproveSelection(search, search.remainingAt(lotIndex), totals) {
		return nil
	}
	if lotIndex == len(search.lots) {
		evaluateSelection(search, totals)
		return nil
	}

	lot := &search.lots[lotIndex]
	asset, sell := lot.asset, lot.sell
	minQuantity, maxQuantity := quantityBounds(search, lotIndex, totals.proceeds)
	one := decimal.NewFromInt(1)
	for quantity := maxQuantity; quantity.GreaterThanOrEqual(minQuantity); quantity = quantity.Sub(one) {
		amount := quantity.Mul(sell.Price)
		lot.quantity = quantity
		previousSold := asset.sold
		asset.sold = previousSold.Add(amount)
		next := totals
		next.proceeds = next.proceeds.Add(amount)
		next.tax = next.tax.Add(quantity.Mul(lot.taxPerShare))
		if asset.candidate.Asset.Plan == nil {
			next.unplanned = next.unplanned.Add(amount)
		}
		err := searchSales(ctx, search, lotIndex+1, next)
		asset.sold = previousSold
		if err != nil {
			return err
		}
	}
	return nil
}

func canImproveSelection(search *saleSearch, remaining, totals saleTotals) bool {
	if totals.proceeds.Add(remaining.proceeds).LessThan(search.funding.MinTargetSale) {
		return false
	}
	if !search.found {
		return true
	}
	possible := saleTotals{
		tax:      totals.tax.Add(remaining.tax),
		proceeds: decimal.Max(totals.proceeds, search.funding.MinTargetSale),
		unplanned: totals.unplanned.Add(decimal.Min(
			remaining.unplanned, search.funding.MaxTargetSale.Sub(totals.proceeds),
		)),
	}
	return compareSaleTotals(possible, search.bestTotals) > 0
}

func evaluateSelection(search *saleSearch, totals saleTotals) {
	order := compareSaleTotals(totals, search.bestTotals)
	if search.found && order <= 0 {
		return
	}
	remainingValue := search.totalValue.Sub(totals.proceeds)
	for _, asset := range search.assets {
		minSale, maxSale, err := selectionBounds(asset, remainingValue)
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

func selectionBounds(asset searchAsset, remaining decimal.Decimal) (decimal.Decimal, decimal.Decimal, error) {
	return asset.weights.saleBounds(asset.candidate.Asset, asset.candidate.Value, remaining)
}

func (search *saleSearch) remainingAt(lotIndex int) saleTotals {
	if lotIndex == len(search.lots) {
		return saleTotals{}
	}
	return search.lots[lotIndex].remaining
}

func quantityBounds(search *saleSearch, lotIndex int, proceeds decimal.Decimal) (minQuantity, maxQuantity decimal.Decimal) {
	lot := &search.lots[lotIndex]
	asset, sell := lot.asset, lot.sell
	capacity := decimal.Min(search.funding.MaxTargetSale.Sub(proceeds), asset.saleLimit.Sub(asset.sold))
	maxQuantity, _ = capacity.QuoRem(sell.Price, 0)
	maxQuantity = decimal.Min(maxQuantity, lot.available)
	needed := search.funding.MinTargetSale.Sub(proceeds).Sub(search.remainingAt(lotIndex + 1).proceeds)
	minQuantity, remainder := decimal.Max(decimal.Zero, needed).QuoRem(sell.Price, 0)
	if remainder.IsPositive() {
		minQuantity = minQuantity.Add(decimal.NewFromInt(1))
	}
	return minQuantity, maxQuantity
}

func assembleSales(lots []searchLot) []orders.SellProposal {
	var proposals []orders.SellProposal
	byTicker := make(map[string]int)
	for _, lot := range lots {
		if lot.best.IsZero() {
			continue
		}
		sell := *lot.sell
		sell.Quantity = lot.best
		index, exists := byTicker[sell.Ticker]
		if !exists {
			index = len(proposals)
			byTicker[sell.Ticker] = index
			proposals = append(proposals, orders.SellProposal{Ticker: sell.Ticker, Price: sell.Price, Type: orders.Market})
		}
		proposals[index].Add(sell)
	}
	return proposals
}
