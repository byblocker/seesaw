package rebalancer

import (
	"fmt"
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type buyRequest struct {
	Ticker string
	Amount decimal.Decimal
	Ask    decimal.Decimal
}

func selectBuy(snapshot portfolio.Snapshot, asset portfolio.Asset, sell portfolio.SellProposal, shortfall decimal.Decimal) (buyRequest, bool) {
	for _, idx := range asset.PositionIdxs {
		position := snapshot.Positions[idx]
		if (sell.IsEmpty() || idx != sell.PositionIdx) && !position.RecentlySold {
			return buyRequest{
				Ticker: position.Ticker,
				Amount: shortfall,
				Ask:    position.Quote.Ask,
			}, true
		}
	}

	return buyRequest{}, false
}

func sizeBuys(
	snapshot portfolio.Snapshot,
	settings config.Settings,
	broker models.BrokerSettings,
	sells []portfolio.SellProposal,
	requests []buyRequest,
	maxValueDeviation decimal.Decimal,
) []portfolio.Buy {
	balance := snapshot.Balances.BuyingPower
	for _, sell := range sells {
		balance = balance.Add(sell.Proceeds)
	}

	log.Printf("Sizing buy requests with %s available . . .", utils.FormatMoney(balance))
	var buys []portfolio.Buy
	for _, request := range requests {
		if !balance.IsPositive() {
			log.Print("SKIP remaining buy requests: no buying power left")
			break
		}

		buy := proposeBuy(
			request,
			balance,
			maxValueDeviation,
			settings.MinFractionalShareToRound,
			broker.AllowFractionalShares,
		)
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
	return buys
}

func proposeBuy(
	req buyRequest,
	balance,
	maxValueDeviation,
	minFractionalShareToRound decimal.Decimal,
	allowFractionalShares bool,
) portfolio.Buy {
	valueToCover := decimal.Min(req.Amount, balance)
	exact, _ := valueToCover.QuoRem(req.Ask, utils.DivisionDecimalPlaces)
	if allowFractionalShares {
		return req.buy(exact)
	}

	whole := exact.Floor()
	fractional := exact.Sub(whole)
	if fractional.IsZero() {
		return req.buy(whole)
	}

	// Truncate so the shown count never rounds up to the whole share it falls short of.
	cannotRound := fmt.Sprintf("CANNOT round up %s from %s share(s)", req.Ticker, exact.Truncate(2))
	if fractional.LessThan(minFractionalShareToRound) {
		log.Printf("%s: below rounding threshold", cannotRound)
		return req.buy(whole)
	}

	rounded := whole.Add(decimal.NewFromInt(1))
	cost := rounded.Mul(req.Ask)
	if cost.GreaterThan(balance) {
		log.Printf("%s: not enough funds", cannotRound)
		return req.buy(whole)
	}
	maxAllowedCost := req.Amount.Add(maxValueDeviation)
	if cost.GreaterThan(maxAllowedCost) {
		log.Printf("%s: exceeds risk threshold", cannotRound)
		return req.buy(whole)
	}

	log.Printf("CAN round up %s from %s to %s share(s)", req.Ticker, exact.Truncate(2), rounded)
	return req.buy(rounded)
}

func (req buyRequest) buy(quantity decimal.Decimal) portfolio.Buy {
	return portfolio.Buy{
		Ticker:        req.Ticker,
		Price:         req.Ask,
		Quantity:      quantity,
		EstimatedCost: quantity.Mul(req.Ask),
	}
}
