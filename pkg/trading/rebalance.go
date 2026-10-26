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
	client models.BrokerClient,
	request models.RebalanceRequest,
) (*models.RebalancePlan, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if !snapshot.TotalValue.IsPositive() {
		return nil, fmt.Errorf("empty or negative portfolio")
	}

	plan, err := rebalancer.Plan(snapshot, request, client.Settings())
	if err != nil {
		return nil, err
	}

	log.Printf("Rebalance complete: %d buy(s), %d sell(s)", len(plan.Buys), len(plan.Sells))
	return plan, nil
}
