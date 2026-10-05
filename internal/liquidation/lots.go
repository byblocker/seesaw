package liquidation

import (
	"cmp"
	"fmt"
	"seesaw/pkg/models"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

type LiquidateCandidate struct {
	Asset *models.Asset
	Lots  []models.SellLot
}

func RankLots(
	snapshot models.AccountSnapshot,
	quotes map[string]models.Quote,
	selection models.LotSelection,
	rates *models.TaxSettings,
	saleDate time.Time,
) ([]LiquidateCandidate, error) {
	if selection != models.EstimatedTax && selection != models.MinTaxOrder {
		return nil, fmt.Errorf("unsupported lot selection %q", selection)
	}
	if snapshot.Portfolio == nil {
		return nil, fmt.Errorf("snapshot is missing portfolio")
	}

	groups := make([]LiquidateCandidate, 0, len(snapshot.Assets))
	for i := range snapshot.Assets {
		asset := &snapshot.Assets[i]
		group := LiquidateCandidate{Asset: asset}
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

				group.Lots = append(group.Lots, models.NewSellLot(ticker, lot, quote.Bid, saleDate))
			}
		}
		slices.SortFunc(group.Lots, func(a, b models.SellLot) int {
			return compareLots(a, b, selection, rates)
		})
		groups = append(groups, group)
	}

	slices.SortStableFunc(groups, func(a, b LiquidateCandidate) int {
		return cmp.Compare(assetRank(a.Asset), assetRank(b.Asset))
	})
	return groups, nil
}

const (
	unplannedAssetRank = iota
	plannedAssetRank
)

func assetRank(asset *models.Asset) int {
	if asset.Plan == nil {
		return unplannedAssetRank
	}
	return plannedAssetRank
}

func compareLots(a, b models.SellLot, selection models.LotSelection, rates *models.TaxSettings) int {
	if rates != nil {
		if selection == models.EstimatedTax {
			aTax, bTax := a.TaxImpactPerShare(*rates), b.TaxImpactPerShare(*rates)
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
