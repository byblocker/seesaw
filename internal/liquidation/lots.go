package liquidation

import (
	"cmp"
	"fmt"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"time"

	"github.com/shopspring/decimal"
)

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

const (
	unplannedAssetRank = iota
	plannedAssetRank
)

func assetRank(asset *portfolio.Asset) int {
	if asset.Plan == nil {
		return unplannedAssetRank
	}
	return plannedAssetRank
}

func compareLots(a, b models.SellLot, aTax, bTax decimal.Decimal, taxable bool) int {
	if taxable {
		if !aTax.IsZero() || !bTax.IsZero() {
			// Quantity cancels out of tax per dollar; cross multiplication avoids rounding.
			if order := aTax.Mul(b.Price).Cmp(bTax.Mul(a.Price)); order != 0 {
				return order
			}
		}
		if order := cmp.Compare(taxCategory(a.GainPerShare, a.LongTerm), taxCategory(b.GainPerShare, b.LongTerm)); order != 0 {
			return order
		}
		if order := a.GainPerShare.Mul(b.Price).Cmp(b.GainPerShare.Mul(a.Price)); order != 0 {
			return order
		}
	}

	if order := cmp.Compare(a.Ticker, b.Ticker); order != 0 {
		return order
	}

	return cmp.Compare(a.Lot.Id, b.Lot.Id)
}

const (
	shortTermLossRank = iota
	longTermLossRank
	breakEvenRank
	longTermGainRank
	shortTermGainRank
)

func taxCategory(gain decimal.Decimal, longTerm bool) int {
	sign := gain.Sign()
	switch {
	case sign < 0 && !longTerm:
		return shortTermLossRank
	case sign < 0:
		return longTermLossRank
	case sign == 0:
		return breakEvenRank
	case longTerm:
		return longTermGainRank
	default:
		return shortTermGainRank
	}
}
