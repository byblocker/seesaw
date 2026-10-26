package trading

import (
	"context"
	"log"
	"seesaw/internal/liquidation"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
)

func Liquidate(
	ctx context.Context,
	snapshot portfolio.Snapshot,
	request models.LiquidationRequest,
) (*models.LiquidationSelection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}

	selection, err := liquidation.PlanSales(snapshot, request)
	if err != nil {
		return nil, err
	}

	log.Printf("Liquidation planning complete: %d sell(s)", len(selection.Sells))
	return selection, nil
}
