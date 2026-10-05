package liquidation

import (
	"cmp"
	"fmt"
	"seesaw/pkg/models"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

type LotCandidate struct {
	Ticker            string
	AssetIndex        int
	Lot               *models.PositionLot
	Price             decimal.Decimal
	Unplanned         bool
	LongTerm          bool
	PotentialWashSale bool
	Proceeds          decimal.Decimal
	RealizedPL        decimal.Decimal
	EstimatedTax      decimal.Decimal // Optional
}

type LotSelector struct {
	strategy     models.LotSelection
	taxable      bool
	saleDate     time.Time
	taxEstimator *TaxEstimator
}

func NewLotSelector(
	strategy models.LotSelection,
	taxable bool,
	rates models.TaxSettings,
	saleDate time.Time,
) (LotSelector, error) {
	if strategy != models.EstimatedTax && strategy != models.MinTaxOrder {
		return LotSelector{}, fmt.Errorf("unsupported lot selection %q", strategy)
	}

	selector := LotSelector{strategy: strategy, taxable: taxable, saleDate: saleDate}
	if taxable && strategy == models.EstimatedTax {
		estimator, err := NewTaxEstimator(rates, saleDate)
		if err != nil {
			return LotSelector{}, err
		}

		selector.taxEstimator = &estimator
	}

	return selector, nil
}

func (selector LotSelector) RankLots(
	snapshot models.AccountSnapshot,
	quotes map[string]models.Quote,
) ([]LotCandidate, error) {
	if snapshot.Portfolio == nil {
		return nil, fmt.Errorf("snapshot is missing portfolio")
	}

	var candidates []LotCandidate
	for assetIndex, asset := range snapshot.Assets {
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

				sale := models.SellProposal{Ticker: ticker, Type: models.Market, Price: quote.Bid}
				sale.Add(models.SellLot{Lot: *lot, Quantity: lot.Quantity})
				candidate := LotCandidate{
					Ticker: ticker, AssetIndex: assetIndex, Lot: lot, Price: quote.Bid,
					Unplanned: asset.Plan == nil,
					LongTerm:  lot.IsLongTermAt(selector.saleDate),
					Proceeds:  sale.Proceeds, RealizedPL: sale.RealizedPL,
				}

				candidate.PotentialWashSale = selector.hasPotentialWashSale(sale, snapshot.Portfolio.RecentTx)

				if selector.taxEstimator != nil {
					tax, err := selector.taxEstimator.Estimate(sale)
					if err != nil {
						return nil, fmt.Errorf("estimate tax for %s lot %d: %w", ticker, lot.Id, err)
					}

					candidate.EstimatedTax = tax
				}

				candidates = append(candidates, candidate)
			}
		}
	}

	slices.SortFunc(candidates, selector.compare)
	return candidates, nil
}

func (selector LotSelector) hasPotentialWashSale(sale models.SellProposal, recentTx models.RecentTxByTicker) bool {
	return selector.taxable && sale.RealizedPL.IsNegative() && len(recentTx[sale.Ticker].Buys) > 0
}

func (selector LotSelector) compare(a, b LotCandidate) int {
	if a.Unplanned != b.Unplanned {
		if a.Unplanned {
			return -1
		}
		return 1
	}
	if selector.taxable {
		if selector.strategy == models.EstimatedTax {
			// Cross multiplication compares tax per dollar without division rounding.
			if order := a.EstimatedTax.Mul(b.Proceeds).Cmp(b.EstimatedTax.Mul(a.Proceeds)); order != 0 {
				return order
			}
		}
		if order := cmp.Compare(taxCategory(a), taxCategory(b)); order != 0 {
			return order
		}
		if order := a.RealizedPL.Mul(b.Proceeds).Cmp(b.RealizedPL.Mul(a.Proceeds)); order != 0 {
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

func taxCategory(candidate LotCandidate) int {
	sign := candidate.RealizedPL.Sign()
	switch {
	case sign < 0 && !candidate.LongTerm:
		return shortTermLossRank
	case sign < 0:
		return longTermLossRank
	case sign == 0:
		return breakEvenRank
	case candidate.LongTerm:
		return longTermGainRank
	default:
		return shortTermGainRank
	}
}
