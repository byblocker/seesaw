package trading

import (
	"context"
	"fmt"
	"log"
	"seesaw/internal/liquidation"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"time"
)

func Liquidate(
	ctx context.Context,
	snapshot portfolio.Snapshot,
	client models.BrokerClient,
	request models.LiquidationRequest,
) (*models.LiquidationSelection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if snapshot.Portfolio == nil {
		return nil, fmt.Errorf("snapshot is missing portfolio")
	}

	plan := liquidation.PlanFunding(snapshot.Portfolio.Balances, request)

	if plan.MinTargetSale.IsZero() {
		return &models.LiquidationSelection{
			Plan:            plan,
			ExpectedBalance: plan.StartingCash.Sub(request.Amount),
		}, nil
	}

	saleDate := time.Now()
	quotes, err := client.QuoteAll(ctx, snapshot.Portfolio.ActiveTickers())
	if err != nil {
		return nil, err
	}

	log.Printf(
		"Selecting liquidation sales between $%s and $%s",
		plan.MinTargetSale.StringFixed(2), plan.MaxTargetSale.StringFixed(2),
	)

	selection, err := liquidation.PlanSales(
		ctx,
		snapshot,
		quotes,
		saleDate,
		plan,
	)
	if err != nil {
		return nil, err
	}

	log.Printf("Liquidation planning complete: %d sell(s)", len(selection.Sells))
	return selection, nil
}
