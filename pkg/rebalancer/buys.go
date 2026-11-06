package rebalancer

import (
	"fmt"
	"log"
	"seesaw/internal/format"
	"seesaw/pkg/config"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type buyRequest struct {
	PositionIdx int
	Amount      decimal.Decimal
}

func sizeBuys(
	snapshot portfolio.Snapshot,
	settings config.Settings,
	sells []portfolio.SellProposal,
	buyRequests []buyRequest,
	maxValueDeviation decimal.Decimal,
) []portfolio.Buy {
	balance := snapshot.Balances.BuyingPower
	for _, sell := range sells {
		balance = balance.Add(sell.Proceeds)
	}

	log.Printf("Sizing buy requests with %s available . . .", format.Money(balance))
	var buys []portfolio.Buy
	for _, request := range buyRequests {
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
			snapshot.Broker.AllowFractionalShares,
		)
		if buy.Quantity.IsZero() {
			log.Printf("CANNOT buy %s: not enough for one share", ticker)
			continue
		}

		buys = append(buys, buy)
		balance = balance.Sub(buy.EstimatedCost)
		log.Printf("BUY %s share(s) of %s at %s (~%s total)",
			buy.Quantity.String(), ticker, format.Price(buy.Price), format.Money(buy.EstimatedCost))
		log.Printf("Remaining balance: %s", format.Money(balance))
	}
	return buys
}

func proposeBuy(
	snapshot portfolio.Snapshot,
	request buyRequest,
	balance,
	maxValueDeviation,
	minFractionalShareToRound decimal.Decimal,
	allowFractionalShares bool,
) portfolio.Buy {
	position := snapshot.Positions[request.PositionIdx]
	ask := position.Quote.Ask
	buy := func(quantity decimal.Decimal) portfolio.Buy {
		return portfolio.Buy{
			PositionIdx:   request.PositionIdx,
			Price:         ask,
			Quantity:      quantity,
			EstimatedCost: quantity.Mul(ask),
		}
	}

	valueToCover := decimal.Min(request.Amount, balance)
	exactShares, _ := valueToCover.QuoRem(ask, portfolio.DivisionDecimalPlaces)
	if allowFractionalShares {
		return buy(exactShares)
	}

	wholeShares := exactShares.Floor()
	fraction := exactShares.Sub(wholeShares)
	if fraction.IsZero() {
		return buy(wholeShares)
	}

	// Truncate so the shown count never rounds up to the whole share it falls short of.
	cannotRound := fmt.Sprintf("CANNOT round up %s from %s share(s)", position.Ticker, exactShares.Truncate(2))
	if fraction.LessThan(minFractionalShareToRound) {
		log.Printf("%s: below rounding threshold", cannotRound)
		return buy(wholeShares)
	}

	roundedShares := wholeShares.Add(decimal.NewFromInt(1))
	roundedCost := roundedShares.Mul(ask)
	if roundedCost.GreaterThan(balance) {
		log.Printf("%s: not enough funds", cannotRound)
		return buy(wholeShares)
	}
	maxAllowedCost := request.Amount.Add(maxValueDeviation)
	if roundedCost.GreaterThan(maxAllowedCost) {
		log.Printf("%s: exceeds risk threshold", cannotRound)
		return buy(wholeShares)
	}

	log.Printf("CAN round up %s from %s to %s share(s)", position.Ticker, exactShares.Truncate(2), roundedShares)
	return buy(roundedShares)
}
