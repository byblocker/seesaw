package rebalancer

import (
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

	log.Printf("Sizing %d purchase(s) with %s available, including proposed sales", len(requests), utils.FormatMoney(balance))
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
			log.Printf("Skip buying %s: not enough to buy one share", request.Ticker)
			continue
		}

		buys = append(buys, buy)
		balance = balance.Sub(buy.EstimatedCost)
		log.Printf("Buy %s share(s) of %s at %s for %s",
			buy.Quantity.String(), buy.Ticker, utils.FormatPrice(buy.Price), utils.FormatMoney(buy.EstimatedCost))
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

	valueToCover := decimal.Min(req.Amount, balance)
	exact, _ := valueToCover.QuoRem(req.Ask, utils.DivisionDecimalPlaces)
	if allowFractionalShares {
		return req.buy(exact), nil
	}

	whole := exact.Floor()
	fractional := exact.Sub(whole)
	if fractional.IsZero() || fractional.LessThan(minFractionalShareToRound) {
		return req.buy(whole), nil
	}

	rounded := whole.Add(decimal.NewFromInt(1))
	cost := rounded.Mul(req.Ask)
	maxAllowedCost := req.Amount.Add(maxValueDeviation)
	if cost.GreaterThan(balance) || cost.GreaterThan(maxAllowedCost) {
		return req.buy(whole), nil
	}

	return req.buy(rounded), nil
}

func (req BuyRequest) buy(quantity decimal.Decimal) models.Buy {
	return models.Buy{
		Ticker:        req.Ticker,
		Price:         req.Ask,
		Quantity:      quantity,
		EstimatedCost: quantity.Mul(req.Ask),
	}
}
