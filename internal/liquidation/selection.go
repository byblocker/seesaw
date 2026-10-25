package liquidation

import (
	"fmt"

	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

func PlanSales(
	snapshot portfolio.Snapshot,
	plan *models.LiquidationPlan,
) (*models.LiquidationSelection, error) {
	positions, assets := collectPositions(snapshot, plan)

	sellableValue := decimal.Zero
	for i := range positions {
		position := &positions[i]
		sellableValue = sellableValue.Add(position.saleProceeds(position.sellableShares))
	}
	if sellableValue.LessThan(plan.MinTargetSale) {
		return nil, fmt.Errorf(
			"whole-share holdings can raise at most $%s of the $%s needed",
			sellableValue.StringFixed(2), plan.MinTargetSale.StringFixed(2),
		)
	}

	selection, err := selectLots(positions, assets, plan.MinTargetSale)
	if err != nil {
		return nil, err
	}

	var proposals []models.SellProposal
	proposalByTicker := map[string]int{}
	for _, sale := range selection.sales {
		i, ok := proposalByTicker[sale.Ticker]
		if !ok {
			i = len(proposals)
			proposalByTicker[sale.Ticker] = i
			proposals = append(proposals, models.SellProposal{Ticker: sale.Ticker, Price: sale.Price})
		}
		proposals[i].Add(sale)
	}

	totalValueAfterRequest := decimal.Zero.Sub(plan.MinTargetSale)
	for _, asset := range assets {
		totalValueAfterRequest = totalValueAfterRequest.Add(asset.currentValue)
	}

	return &models.LiquidationSelection{
		Plan:               plan,
		Sells:              proposals,
		Proceeds:           selection.proceeds,
		AllocationWarnings: allocationWarnings(assets, selection.soldByAsset, totalValueAfterRequest),
	}, nil
}
