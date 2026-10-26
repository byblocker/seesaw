package liquidation

import (
	"fmt"
	"log"

	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

func PlanSales(
	snapshot portfolio.Snapshot,
	request models.LiquidationRequest,
) (*models.LiquidationSelection, error) {
	cash := snapshot.Balances.CashBalance
	result := &models.LiquidationSelection{Request: request, StartingCash: cash}

	// Use existing cash first, and sell only for the rest of the withdrawal.
	amountNeeded := utils.ClampNonNegative(request.Amount.Sub(cash))
	if amountNeeded.IsZero() {
		return result, nil
	}

	log.Printf("Selecting liquidation sales for at least %s", utils.FormatMoney(amountNeeded))
	lots := collectLots(snapshot, request, amountNeeded)

	sellableValue := decimal.Zero
	for i := range lots {
		sellableValue = sellableValue.Add(lots[i].saleProceeds(lots[i].wholeShares))
	}
	if sellableValue.LessThan(amountNeeded) {
		return nil, fmt.Errorf(
			"whole-share holdings can raise at most $%s of the $%s needed",
			sellableValue.StringFixed(2), amountNeeded.StringFixed(2),
		)
	}

	selection, err := selectLots(lots, amountNeeded)
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

	result.Sells = proposals
	result.Proceeds = selection.proceeds
	return result, nil
}
