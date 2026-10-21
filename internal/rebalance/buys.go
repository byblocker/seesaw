package rebalance

import (
	"fmt"
	"seesaw/internal/utils"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

type BuyRequest struct {
	Ticker string
	Amount decimal.Decimal
	Ask    decimal.Decimal
}

func ProposeBuy(
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
