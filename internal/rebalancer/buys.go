package rebalancer

import (
	"context"
	"fmt"
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"time"

	"github.com/shopspring/decimal"
)

type BuyRequest struct {
	Ticker string
	Amount decimal.Decimal
	Ask    decimal.Decimal
}

// The buy that covers what an asset's sale leaves it short: its first ticker that isn't being
// sold and wasn't sold in the wash-sale window.
func selectBuy(
	asset portfolio.Asset,
	recentTx models.RecentTxByTicker,
	tradeDate time.Time,
	sell models.SellProposal,
	quoteByTicker map[string]models.Quote,
) (BuyRequest, bool) {
	shortfall := sell.Proceeds.Sub(asset.ValueDrift)
	if !shortfall.IsPositive() {
		// We should not buy into overweight position
		return BuyRequest{}, false
	}

	for ticker := range asset.Plan.Tickers() {
		isBeingSold := len(sell.Lots) > 0 && ticker == sell.Ticker
		if !isBeingSold && !recentTx.HasSellWithinWashWindow(ticker, tradeDate) {
			return BuyRequest{
				Ticker: ticker,
				Amount: shortfall,
				Ask:    quoteByTicker[ticker].Ask,
			}, true
		}
	}

	return BuyRequest{}, false
}

// Sizes each buy against the cash left after the buys before it, counting the sales' proceeds.
func (r *Rebalancer) SizeBuys(
	ctx context.Context,
	client models.BrokerClient,
	snapshot portfolio.Snapshot,
	sells []models.SellProposal,
	requests []BuyRequest,
) ([]models.Buy, error) {
	balance := snapshot.Portfolio.Balances.BuyingPower
	for _, sell := range sells {
		balance = balance.Add(sell.Proceeds)
	}
	maxValueDeviationAllowed := snapshot.Portfolio.TotalValue.Mul(*r.settings.MaxPctAlocDeviation)
	allowFractionalShares := client.Settings().AllowFractionalShares

	log.Printf("Sizing %d purchase(s) with $%s available, including proposed sales", len(requests), balance.StringFixed(2))
	var buys []models.Buy
	for _, request := range requests {
		if balance.Cmp(decimal.Zero) <= 0 {
			log.Print("No buying power remains; skipping remaining purchases")
			break
		}

		buy, err := proposeBuy(
			request,
			balance,
			maxValueDeviationAllowed,
			*r.settings.MinFractionalShareToRound,
			allowFractionalShares,
		)
		if err != nil {
			return nil, fmt.Errorf("size buy for %s: %w", request.Ticker, err)
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
	return buys, nil
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
	if !req.Amount.IsPositive() || !balance.IsPositive() {
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
