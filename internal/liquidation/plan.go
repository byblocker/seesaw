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
) ([]models.SellProposal, error) {
	// Use existing cash first, and sell only for the rest of the withdrawal.
	amountNeeded := utils.ClampNonNegative(request.Amount.Sub(snapshot.Balances.CashBalance))
	if amountNeeded.IsZero() {
		return nil, nil
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

	sales, err := selectLots(lots, amountNeeded)
	if err != nil {
		return nil, err
	}

	return models.GroupSales(sales), nil
}
