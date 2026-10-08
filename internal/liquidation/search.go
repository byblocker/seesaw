package liquidation

import (
	"context"
	"fmt"

	"seesaw/internal/utils"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

type searchAsset struct {
	isUnplanned bool
	value       decimal.Decimal
	minWeight   decimal.Decimal
	maxWeight   decimal.Decimal
	saleLimit   decimal.Decimal
	sold        decimal.Decimal
}

func (asset *searchAsset) saleBounds(remainingInvestedValue decimal.Decimal) (minSale, maxSale decimal.Decimal) {
	if asset.isUnplanned {
		return decimal.Zero, asset.value
	}
	minRemaining := remainingInvestedValue.Mul(asset.minWeight)
	maxRemaining := remainingInvestedValue.Mul(asset.maxWeight)

	minSale = utils.ClampNonNegative(asset.value.Sub(maxRemaining))
	maxSale = asset.value.Sub(minRemaining)

	return minSale, maxSale
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
	asset := lot.asset
	minQuantity, maxQuantity := search.quantityBounds(lotIndex, totals.proceeds)
	one := decimal.NewFromInt(1)
	for quantity := maxQuantity; quantity.GreaterThanOrEqual(minQuantity); quantity = quantity.Sub(one) {
		amount := quantity.Mul(lot.sell.Price)
		lot.quantity = quantity
		previousSold := asset.sold
		asset.sold = previousSold.Add(amount)
		next := totals
		next.proceeds = next.proceeds.Add(amount)
		next.tax = next.tax.Add(quantity.Mul(lot.taxPerShare))
		if asset.isUnplanned {
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
		minSale, maxSale := asset.saleBounds(remainingValue)
		if asset.sold.LessThan(minSale) || asset.sold.GreaterThan(maxSale) {
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
	asset := lot.asset
	capacity := decimal.Min(search.maxTargetSale.Sub(proceeds), asset.saleLimit.Sub(asset.sold))
	maxQuantity, _ = capacity.QuoRem(lot.sell.Price, 0)
	maxQuantity = decimal.Min(maxQuantity, lot.available)
	needed := search.minTargetSale.Sub(proceeds).Sub(search.remainingAt(lotIndex + 1).proceeds)
	minQuantity, remainder := decimal.Max(decimal.Zero, needed).QuoRem(lot.sell.Price, 0)
	if remainder.IsPositive() {
		minQuantity = minQuantity.Add(decimal.NewFromInt(1))
	}
	return minQuantity, maxQuantity
}
