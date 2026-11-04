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
	PositionIdx int
	Amount      decimal.Decimal
}

func selectBuy(snapshot portfolio.Snapshot, asset portfolio.Asset, sell portfolio.SellProposal, shortfall decimal.Decimal) (buyRequest, bool) {
	soldPositionIdx := -1
	if !sell.IsEmpty() {
		soldPositionIdx = sell.PositionIdx
	}
	positionIdx, ok := replacement(snapshot, asset, soldPositionIdx)
	return buyRequest{PositionIdx: positionIdx, Amount: shortfall}, ok
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

		ticker := snapshot.Positions[request.PositionIdx].Ticker
		buy := proposeBuy(
			snapshot,
			request,
			balance,
			maxValueDeviation,
			settings.MinFractionalShareToRound,
			broker.AllowFractionalShares,
		)
		if buy.Quantity.IsZero() {
			log.Printf("CANNOT buy %s: not enough for one share", ticker)
			continue
		}

		buys = append(buys, buy)
		balance = balance.Sub(buy.EstimatedCost)
		log.Printf("BUY %s share(s) of %s at %s (~%s total)",
			buy.Quantity.String(), ticker, utils.FormatPrice(buy.Price), utils.FormatMoney(buy.EstimatedCost))
		log.Printf("Remaining balance: %s", utils.FormatMoney(balance))
	}
	return buys
}

func proposeBuy(
	snapshot portfolio.Snapshot,
	req buyRequest,
	balance,
	maxValueDeviation,
	minFractionalShareToRound decimal.Decimal,
	allowFractionalShares bool,
) portfolio.Buy {
	position := snapshot.Positions[req.PositionIdx]
	ask := position.Quote.Ask
	buy := func(quantity decimal.Decimal) portfolio.Buy {
		return portfolio.Buy{
			PositionIdx:   req.PositionIdx,
			Price:         ask,
			Quantity:      quantity,
			EstimatedCost: quantity.Mul(ask),
		}
	}

	valueToCover := decimal.Min(req.Amount, balance)
	exact, _ := valueToCover.QuoRem(ask, utils.DivisionDecimalPlaces)
	if allowFractionalShares {
		return buy(exact)
	}

	whole := exact.Floor()
	fractional := exact.Sub(whole)
	if fractional.IsZero() {
		return buy(whole)
	}

	// Truncate so the shown count never rounds up to the whole share it falls short of.
	cannotRound := fmt.Sprintf("CANNOT round up %s from %s share(s)", position.Ticker, exact.Truncate(2))
	if fractional.LessThan(minFractionalShareToRound) {
		log.Printf("%s: below rounding threshold", cannotRound)
		return buy(whole)
	}

	rounded := whole.Add(decimal.NewFromInt(1))
	cost := rounded.Mul(ask)
	if cost.GreaterThan(balance) {
		log.Printf("%s: not enough funds", cannotRound)
		return buy(whole)
	}
	maxAllowedCost := req.Amount.Add(maxValueDeviation)
	if cost.GreaterThan(maxAllowedCost) {
		log.Printf("%s: exceeds risk threshold", cannotRound)
		return buy(whole)
	}

	log.Printf("CAN round up %s from %s to %s share(s)", position.Ticker, exact.Truncate(2), rounded)
	return buy(rounded)
}
