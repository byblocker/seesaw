package liquidation

import (
	"context"
	"fmt"
	"time"

	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
)

func PlanSales(
	ctx context.Context,
	snapshot portfolio.Snapshot,
	quotes map[string]models.Quote,
	saleDate time.Time,
	plan *models.LiquidationPlan,
) (*models.LiquidationSelection, error) {
	search, err := prepareSaleSearch(
		snapshot,
		quotes,
		saleDate,
		plan,
	)
	if err != nil {
		return nil, err
	}
	if err := search.visit(ctx, 0, saleTotals{}); err != nil {
		return nil, err
	}
	if !search.found {
		return nil, fmt.Errorf("no whole-share liquidation meets the funding and allocation limits")
	}

	return &models.LiquidationSelection{
		Plan:            plan,
		ExpectedBalance: plan.StartingCash.Sub(plan.Request.Amount).Add(search.bestTotals.proceeds),
		Sells:           assembleSales(search.lots),
		Proceeds:        search.bestTotals.proceeds,
	}, nil
}

func assembleSales(lots []searchLot) []models.SellProposal {
	var proposals []models.SellProposal
	byTicker := make(map[string]int)
	for _, lot := range lots {
		if lot.best.IsZero() {
			continue
		}

		sell := lot.sell
		sell.Quantity = lot.best

		index, exists := byTicker[sell.Ticker]
		if !exists {
			index = len(proposals)
			byTicker[sell.Ticker] = index
			proposals = append(proposals, models.SellProposal{Ticker: sell.Ticker, Price: sell.Price, Type: models.Market})
		}

		proposals[index].Add(sell)
	}
	return proposals
}
