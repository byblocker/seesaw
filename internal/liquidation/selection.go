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
	search, err := prepSearch(
		plan,
		snapshot,
		quotes,
		saleDate,
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

	byTicker := make(map[string]models.SellProposal)
	tickers := []string{}
	for _, lot := range search.lots {
		if lot.best.IsZero() {
			continue
		}

		sell := lot.sell
		sell.Quantity = lot.best

		proposal, exists := byTicker[sell.Ticker]
		if !exists {
			proposal = models.SellProposal{Ticker: sell.Ticker, Price: sell.Price, Type: models.Market}
			tickers = append(tickers, sell.Ticker)
		}
		proposal.Add(sell)
		byTicker[sell.Ticker] = proposal
	}

	var proposals []models.SellProposal
	for _, ticker := range tickers {
		proposals = append(proposals, byTicker[ticker])
	}

	return &models.LiquidationSelection{
		Plan:            plan,
		ExpectedBalance: plan.StartingCash.Sub(plan.Request.Amount).Add(search.bestTotals.proceeds),
		Sells:           proposals,
		Proceeds:        search.bestTotals.proceeds,
	}, nil
}
