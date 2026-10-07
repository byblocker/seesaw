package rebalance

import (
	"fmt"
	"seesaw/internal/utils"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

type BuyRequest struct {
	Ticker     string
	ValueDrift decimal.Decimal
	Price      decimal.Decimal
}

func ProposeBuy(
	req BuyRequest,
	balance,
	maxValueDeviation,
	minFractionalShareToRound decimal.Decimal,
	allowFractionalShares bool,
) (models.Buy, error) {
	if !req.Price.IsPositive() {
		return models.Buy{}, fmt.Errorf("price must be positive")
	}
	if !req.ValueDrift.IsPositive() || !balance.IsPositive() {
		return models.Buy{}, nil
	}

	proposeQuantity := func(quantity decimal.Decimal) models.Buy {
		if quantity.IsZero() {
			return models.Buy{}
		}

		return models.Buy{
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

	exact, _ := valueToCover.QuoRem(req.Price, utils.DivisionDecimalPlaces)
	if allowFractionalShares {
		return proposeQuantity(exact), nil
	}

	whole := exact.Floor()
	fractional := exact.Sub(whole)
	if fractional.IsZero() || fractional.LessThan(minFractionalShareToRound) {
		return proposeQuantity(whole), nil
	}

	rounded := whole.Add(decimal.NewFromInt(1))
	cost := rounded.Mul(req.Price)
	maxAllowedCost := req.ValueDrift.Add(maxValueDeviation)

	if cost.Cmp(balance) > 0 || cost.Cmp(maxAllowedCost) > 0 {
		return proposeQuantity(whole), nil
	}

	return proposeQuantity(rounded), nil
}
