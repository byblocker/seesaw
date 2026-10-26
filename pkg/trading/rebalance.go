package trading

import (
	"fmt"
	"log"
	"seesaw/internal/rebalancer"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
)

func Rebalance(
	snapshot portfolio.Snapshot,
	client models.BrokerClient,
	settings config.Settings,
) (*models.RebalancePlan, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	if !snapshot.TotalValue.IsPositive() {
		return nil, fmt.Errorf("empty or negative portfolio")
	}

	plan, err := rebalancer.Plan(snapshot, settings, client.Settings())
	if err != nil {
		return nil, err
	}

	log.Printf("Rebalance complete: %d buy(s), %d sell(s)", len(plan.Buys), len(plan.Sells))
	return plan, nil
}
