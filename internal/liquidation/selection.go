package liquidation

import (
	"context"
	"fmt"

	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
)

func PlanSales(
	ctx context.Context,
	snapshot portfolio.Snapshot,
	plan *models.LiquidationPlan,
) (*models.LiquidationSelection, error) {
	positions, assets := collectPositions(snapshot, plan)

	search, err := newSaleSearch(positions, assets, plan)
	if err != nil {
		return nil, err
	}

	if err := search.run(ctx); err != nil {
		return nil, err
	}
	if !search.found {
		return nil, fmt.Errorf(
			"no whole-share sale raises between $%s and $%s; increase liquidation.max_excess_cash",
			search.minSale.StringFixed(2), search.maxSale.StringFixed(2),
		)
	}

	var proposals []models.SellProposal
	for i, shares := range search.bestSharesByPosition {
		if shares == 0 {
			continue
		}
		position := &positions[i]
		proposal := models.SellProposal{
			Ticker: position.ticker,
			Price:  position.bid,
			Type:   models.Market,
		}
		for _, sale := range position.splitSaleAcrossLots(shares) {
			proposal.Add(sale)
		}
		proposals = append(proposals, proposal)
	}

	proceeds := search.bestTotals.proceeds
	return &models.LiquidationSelection{
		Plan:               plan,
		ExpectedBalance:    plan.BalanceAfter(proceeds),
		Sells:              proposals,
		Proceeds:           proceeds,
		AllocationWarnings: search.allocationWarnings(),
	}, nil
}
