package orders

import (
	"fmt"
	"seesaw/internal/config"

	"github.com/shopspring/decimal"
)

type BuyRequest struct {
	Ticker     string
	ValueDrift decimal.Decimal
	Price      decimal.Decimal
}

type Buy struct {
	Ticker        string
	Price         decimal.Decimal
	Quantity      decimal.Decimal
	EstimatedCost decimal.Decimal
}

func ProposeBuy(
	req BuyRequest,
	balance,
	maxValueDeviation,
	minFractionalShareToRound decimal.Decimal,
	allowFractionalShares bool,
) (Buy, error) {
	if !req.Price.IsPositive() {
		return Buy{}, fmt.Errorf("price must be positive")
	}
	if !req.ValueDrift.IsPositive() || !balance.IsPositive() {
		return Buy{}, nil
	}

	proposeQuantity := func(quantity decimal.Decimal) Buy {
		if quantity.IsZero() {
			return Buy{}
		}

		return Buy{
			Ticker:        req.Ticker,
			Price:         req.Price,
			Quantity:      quantity,
			EstimatedCost: quantity.Mul(req.Price),
		}
	}

	valueToCover := req.ValueDrift
	if balance.LessThan(valueToCover) {
		valueToCover = balance
	}

	// Truncate division to preserve the previous decimal library's behavior.
	exact, _ := valueToCover.QuoRem(req.Price, config.DivisionDecimalPlaces)
	if allowFractionalShares {
		return proposeQuantity(exact), nil
	}

	whole := exact.Floor()
	fractional := exact.Sub(whole)
	if fractional.IsZero() || fractional.LessThan(minFractionalShareToRound) {
		return proposeQuantity(whole), nil
	}

	rounded := whole.Add(config.DecimalOne)
	cost := rounded.Mul(req.Price)
	maxAllowedCost := req.ValueDrift.Add(maxValueDeviation)

	if cost.Cmp(balance) > 0 || cost.Cmp(maxAllowedCost) > 0 {
		return proposeQuantity(whole), nil
	}

	return proposeQuantity(rounded), nil
}
