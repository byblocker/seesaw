package trading

import (
	"log"
	"seesaw/internal/liquidation"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
)

func Liquidate(
	snapshot portfolio.Snapshot,
	request models.LiquidationRequest,
) ([]models.SellProposal, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}

	sells, err := liquidation.PlanSales(snapshot, request)
	if err != nil {
		return nil, err
	}

	log.Printf("Liquidation planning complete: %d sell(s)", len(sells))
	return sells, nil
}
