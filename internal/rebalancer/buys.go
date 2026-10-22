package rebalancer

import (
	"context"
	"fmt"
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type BuyRequest struct {
	Ticker string
	Amount decimal.Decimal
	Ask    decimal.Decimal
}

func selectBuy(asset portfolio.Asset, sell models.SellProposal, shortfall decimal.Decimal) (BuyRequest, bool) {
	for _, position := range asset.Positions {
		if position.Ticker != sell.Ticker && !position.RecentlySold {
			return BuyRequest{
				Ticker: position.Ticker,
				Amount: shortfall,
				Ask:    position.Quote.Ask,
			}, true
		}
	}

	return BuyRequest{}, false
}

func (r *Rebalancer) SizeBuys(
	ctx context.Context,
	client models.BrokerClient,
	snapshot portfolio.Snapshot,
	sells []models.SellProposal,
	requests []BuyRequest,
) ([]models.Buy, decimal.Decimal, error) {
	balance := snapshot.Balances.BuyingPower
	for _, sell := range sells {
		balance = balance.Add(sell.Proceeds)
	}
	maxValueDeviationAllowed := snapshot.TotalValue.Mul(r.settings.MaxPctAllocDeviation)
	allowFractionalShares := client.Settings().AllowFractionalShares

	log.Printf("Sizing %d purchase(s) with $%s available, including proposed sales", len(requests), balance.StringFixed(2))
	var buys []models.Buy
	for _, request := range requests {
		if !balance.IsPositive() {
			log.Print("No buying power remains; skipping remaining purchases")
			break
		}

		buy, err := proposeBuy(
			request,
			balance,
			maxValueDeviationAllowed,
			r.settings.MinFractionalShareToRound,
			allowFractionalShares,
		)
		if err != nil {
			return nil, decimal.Zero, fmt.Errorf("size buy for %s: %w", request.Ticker, err)
		}

		if buy.Quantity.IsZero() {
			utils.Verbosef(ctx, "Skip buying %s: sized quantity is zero under cash and rounding limits", request.Ticker)
			continue
		}

		buys = append(buys, buy)
		balance = balance.Sub(buy.EstimatedCost)
		utils.Verbosef(ctx, "Propose buying %s: %s share(s), cost $%s, remaining balance $%s",
			buy.Ticker, buy.Quantity.String(), buy.EstimatedCost.StringFixed(2), balance.StringFixed(2))
	}
	return buys, balance, nil
}

func proposeBuy(
	req BuyRequest,
	balance,
	maxValueDeviation,
	minFractionalShareToRound decimal.Decimal,
	allowFractionalShares bool,
) (models.Buy, error) {
	if !req.Ask.IsPositive() {
		return models.Buy{}, fmt.Errorf("ask must be positive")
	}
	if !req.Amount.IsPositive() {
		return models.Buy{}, nil
	}

	proposeQuantity := func(quantity decimal.Decimal) models.Buy {
		if quantity.IsZero() {
			return models.Buy{}
		}

		return models.Buy{
			Ticker:        req.Ticker,
			Price:         req.Ask,
			Quantity:      quantity,
			EstimatedCost: quantity.Mul(req.Ask),
		}
	}

	valueToCover := req.Amount
	if balance.LessThan(valueToCover) {
		valueToCover = balance
	}

	exact, _ := valueToCover.QuoRem(req.Ask, utils.DivisionDecimalPlaces)
	if allowFractionalShares {
		return proposeQuantity(exact), nil
	}

	whole := exact.Floor()
	fractional := exact.Sub(whole)
	if fractional.IsZero() || fractional.LessThan(minFractionalShareToRound) {
		return proposeQuantity(whole), nil
	}

	rounded := whole.Add(decimal.NewFromInt(1))
	cost := rounded.Mul(req.Ask)
	maxAllowedCost := req.Amount.Add(maxValueDeviation)

	if cost.Cmp(balance) > 0 || cost.Cmp(maxAllowedCost) > 0 {
		return proposeQuantity(whole), nil
	}

	return proposeQuantity(rounded), nil
}
