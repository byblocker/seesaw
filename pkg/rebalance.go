package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/config"
	"seesaw/internal/orders"
	"seesaw/pkg/etrade"

	"github.com/quagmt/udecimal"
)

func Rebalance(
	ctx context.Context,
	snapshot AccountSnapshot,
	client *etrade.Client,
) ([]orders.BuyOrder, error) {
	if snapshot.TotalValue.LessThanOrEqual(udecimal.Zero) {
		return nil, fmt.Errorf("portfolio value must be positive")
	}

	maxValueDeviation := snapshot.TotalValue.Mul(config.MaxAllocPctDeviationAllowed)
	var buyOrders []orders.BuyOrder

	balance := snapshot.BuyingPower

	for _, asset := range snapshot.Assets {
		if asset.Plan == nil || !asset.ValueDrift.IsNeg() {
			continue
		}

		if balance.LessThanOrEqual(udecimal.Zero) {
			break
		}

		ticker := asset.Plan.PrimaryTicker
		price, err := client.Quote(ctx, ticker)
		if err != nil {
			return nil, fmt.Errorf("retrieve buy price for %s: %w", ticker, err)
		}

		request := orders.BuyRequest{
			ValueDrift:        asset.ValueDrift.Abs(),
			Price:             price,
			MaxValueDeviation: maxValueDeviation,
		}

		quantity, err := orders.CalculateBuyQuantity(request, balance)
		if err != nil {
			return nil, fmt.Errorf("size buy for %s: %w", ticker, err)
		}

		if quantity.IsZero() {
			continue
		}

		cost := quantity.Mul(price)
		buyOrders = append(buyOrders, orders.BuyOrder{
			Ticker:        ticker,
			Price:         price,
			Quantity:      quantity,
			EstimatedCost: cost,
		})
		balance = balance.Sub(cost)
	}
	return buyOrders, nil
}
