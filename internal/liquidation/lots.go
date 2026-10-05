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
	Lots  []LotCandidate
}

type LotCandidate struct {
	Ticker            string
	Lot               *models.PositionLot
	Price             decimal.Decimal
	LongTerm          bool
	PotentialWashSale bool
	Proceeds          decimal.Decimal
	RealizedPL        decimal.Decimal
	EstimatedTax      decimal.Decimal // Optional
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

				sale := models.SellProposal{Ticker: ticker, Type: models.Market, Price: quote.Bid}
				sale.Add(models.SellLot{Lot: *lot, Quantity: lot.Quantity})
				candidate := LotCandidate{
					Ticker: ticker, Lot: lot, Price: quote.Bid,
					LongTerm: lot.IsLongTermAt(saleDate),
					Proceeds: sale.Proceeds, RealizedPL: sale.RealizedPL,
				}

				candidate.PotentialWashSale = rates != nil && hasPotentialWashSale(sale, snapshot.Portfolio.RecentTx)

				if rates != nil && selection == models.EstimatedTax {
					candidate.EstimatedTax = EstimateTax(sale, *rates, saleDate)
				}

				group.Lots = append(group.Lots, candidate)
			}
		}
		slices.SortFunc(group.Lots, func(a, b LotCandidate) int {
			return compareLots(a, b, selection, rates != nil)
		})
		groups = append(groups, group)
	}

	slices.SortStableFunc(groups, func(a, b LiquidateCandidate) int {
		if a.Asset.Plan == nil && b.Asset.Plan != nil {
			return -1
		}
		if b.Asset.Plan == nil && a.Asset.Plan != nil {
			return 1
		}
		return 0
	})
	return groups, nil
}

func hasPotentialWashSale(sale models.SellProposal, recentTx models.RecentTxByTicker) bool {
	return sale.RealizedPL.IsNegative() && recentTx.HasRecentBuy(sale.Ticker)
}

func compareLots(a, b LotCandidate, selection models.LotSelection, taxable bool) int {
	if taxable && selection == models.EstimatedTax {
		// Cross multiplication compares tax per dollar without division rounding.
		if order := a.EstimatedTax.Mul(b.Proceeds).Cmp(b.EstimatedTax.Mul(a.Proceeds)); order != 0 {
			return order
		}
	}

	if taxable {
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
