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
) ([]models.Buy, error) {
	balance := snapshot.Balances.BuyingPower
	for _, sell := range sells {
		balance = balance.Add(sell.Proceeds)
	}
	maxValueDeviationAllowed := snapshot.TotalValue.Mul(r.settings.MaxPctAllocDeviation)
	allowFractionalShares := client.Settings().AllowFractionalShares

	log.Printf("Sizing buy requests with %s available . . .", utils.FormatMoney(balance))
	var buys []models.Buy
	for _, request := range requests {
		if !balance.IsPositive() {
			log.Print("SKIP remaining buy requests: no buying power left")
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
			return nil, fmt.Errorf("size buy for %s: %w", request.Ticker, err)
		}

		if buy.Quantity.IsZero() {
			log.Printf("CANNOT buy %s: not enough for one share", request.Ticker)
			continue
		}

		buys = append(buys, buy)
		balance = balance.Sub(buy.EstimatedCost)
		log.Printf("BUY %s share(s) of %s at %s (~%s total)",
			buy.Quantity.String(), buy.Ticker, utils.FormatPrice(buy.Price), utils.FormatMoney(buy.EstimatedCost))
		log.Printf("Remaining balance: %s", utils.FormatMoney(balance))
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

	valueToCover := decimal.Min(req.Amount, balance)
	exact, _ := valueToCover.QuoRem(req.Ask, utils.DivisionDecimalPlaces)
	if allowFractionalShares {
		return req.buy(exact), nil
	}

	whole := exact.Floor()
	fractional := exact.Sub(whole)
	if fractional.IsZero() {
		return req.buy(whole), nil
	}

	// Truncate so the shown count never rounds up to the whole share it falls short of.
	cannotRound := fmt.Sprintf("CANNOT round up %s from %s share(s)", req.Ticker, exact.Truncate(2))
	if fractional.LessThan(minFractionalShareToRound) {
		log.Printf("%s: below rounding threshold", cannotRound)
		return req.buy(whole), nil
	}

	rounded := whole.Add(decimal.NewFromInt(1))
	cost := rounded.Mul(req.Ask)
	if cost.GreaterThan(balance) {
		log.Printf("%s: not enough funds", cannotRound)
		return req.buy(whole), nil
	}
	maxAllowedCost := req.Amount.Add(maxValueDeviation)
	if cost.GreaterThan(maxAllowedCost) {
		log.Printf("%s: exceeds risk threshold", cannotRound)
		return req.buy(whole), nil
	}

	log.Printf("CAN round up %s from %s to %s share(s)", req.Ticker, exact.Truncate(2), rounded)
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
