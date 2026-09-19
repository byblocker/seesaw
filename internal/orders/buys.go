package orders

import (
	"fmt"
	"seesaw/internal/config"

	"github.com/quagmt/udecimal"
)

type BuyRequest struct {
	ValueDrift        udecimal.Decimal
	Price             udecimal.Decimal
	MaxValueDeviation udecimal.Decimal
}

type BuyOrder struct {
	Ticker        string
	Price         udecimal.Decimal
	Quantity      udecimal.Decimal
	EstimatedCost udecimal.Decimal
}

func CalculateBuyQuantity(
	req BuyRequest,
	balance udecimal.Decimal,
) (udecimal.Decimal, error) {
	if req.Price.LessThanOrEqual(udecimal.Zero) {
		return udecimal.Zero, fmt.Errorf("price must be positive")
	}

	valueToCover := req.ValueDrift
	if balance.LessThan(valueToCover) {
		valueToCover = balance
	}

	exact, _ := valueToCover.Div(req.Price)
	if config.CanBuyFractionalShares {
		return exact, nil
	}

	whole := exact.Floor()
	fractional := exact.Sub(whole)

	if fractional.LessThan(config.MinFractionalShareToRound) {
		return whole, nil
	}

	rounded := whole.Add(udecimal.One)
	estimatedCost := rounded.Mul(req.Price)

	canAfford := balance.GreaterThanOrEqual(estimatedCost)

	maxAllowedCost := req.ValueDrift.Add(req.MaxValueDeviation)
	canAllocateExtra := estimatedCost.LessThanOrEqual(maxAllowedCost)

	if canAfford && canAllocateExtra {
		return rounded, nil
	}

	return whole, nil
}
