package trading

import (
	"fmt"
	"log"
	"seesaw/internal/rebalancer"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
)

func Rebalance(
	snapshot portfolio.Snapshot,
	broker models.BrokerSettings,
	request models.RebalanceRequest,
) (portfolio.RebalancePlan, error) {
	if err := request.Validate(); err != nil {
		return portfolio.RebalancePlan{}, err
	}
	if !snapshot.TotalValue.IsPositive() {
		return portfolio.RebalancePlan{}, fmt.Errorf("empty or negative portfolio")
	}

	plan := rebalancer.Plan(snapshot, request, broker)

	log.Printf("Rebalance complete: %d buy(s), %d sell(s)", len(plan.Buys), len(plan.Sells))
	return plan, nil
}
