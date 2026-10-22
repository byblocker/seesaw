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

	plan := liquidation.PlanFunding(snapshot.Balances, request)

	if plan.MinTargetSale.IsZero() {
		return &models.LiquidationSelection{Plan: plan}, nil
	}

	log.Printf(
		"Selecting liquidation sales between $%s and $%s",
		plan.MinTargetSale.StringFixed(2), plan.MaxTargetSale.StringFixed(2),
	)

	selection, err := liquidation.PlanSales(ctx, snapshot, plan)
	if err != nil {
		return nil, err
	}

	log.Printf("Liquidation planning complete: %d sell(s)", len(selection.Sells))
	return selection, nil
}
