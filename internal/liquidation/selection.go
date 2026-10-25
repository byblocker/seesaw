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
	lots, assets, totalValueAfterSale := collectLots(snapshot, plan)

	sellableValue := decimal.Zero
	for i := range lots {
		sellableValue = sellableValue.Add(lots[i].saleProceeds(lots[i].wholeShares))
	}
	if sellableValue.LessThan(plan.MinTargetSale) {
		return nil, fmt.Errorf(
			"whole-share holdings can raise at most $%s of the $%s needed",
			sellableValue.StringFixed(2), plan.MinTargetSale.StringFixed(2),
		)
	}

	selection, err := selectLots(lots, assets, plan.MinTargetSale)
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

	return &models.LiquidationSelection{
		Plan:               plan,
		Sells:              proposals,
		Proceeds:           selection.proceeds,
		AllocationWarnings: allocationWarnings(assets, selection.soldByAsset, totalValueAfterSale),
	}, nil
}
