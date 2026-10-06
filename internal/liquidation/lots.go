package liquidation

import (
	"cmp"
	"fmt"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

type LiquidationCandidate struct {
	Asset *portfolio.Asset
	Lots  []models.SellLot
	Value decimal.Decimal // Quoted value of all holdings, including fractional shares.
}

func CollectLots(
	snapshot portfolio.Snapshot,
	quotes map[string]models.Quote,
	saleDate time.Time,
) ([]LiquidationCandidate, error) {
	if snapshot.Portfolio == nil {
		return nil, fmt.Errorf("snapshot is missing portfolio")
	}

	candidates := make([]LiquidationCandidate, 0, len(snapshot.Assets))
	for i := range snapshot.Assets {
		asset := &snapshot.Assets[i]
		group := LiquidationCandidate{Asset: asset}
		for _, ticker := range asset.Tickers {
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
				group.Lots = append(group.Lots, sell)
				group.Value = group.Value.Add(sell.Proceeds())
			}
		}
		candidates = append(candidates, group)
	}

	slices.SortStableFunc(candidates, func(a, b LiquidationCandidate) int {
		return cmp.Compare(assetRank(a.Asset), assetRank(b.Asset))
	})
	return candidates, nil
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
