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
) ([]portfolio.SellProposal, error) {
	// Use existing cash first, and sell only for the rest of the withdrawal.
	amountNeeded := utils.ClampNonNegative(request.Amount.Sub(snapshot.Balances.CashBalance))
	if amountNeeded.IsZero() {
		return nil, nil
	}

	log.Printf("Selecting liquidation sales for at least %s", utils.FormatMoney(amountNeeded))
	lotIdxs := sellableLots(snapshot)

	sellableValue := decimal.Zero
	for _, lotIdx := range lotIdxs {
		lot := snapshot.Lots[lotIdx]
		bid := snapshot.Positions[lot.PositionIdx].Quote.Bid
		sellableValue = sellableValue.Add(bid.Mul(decimal.NewFromInt(wholeShares(lot))))
	}
	if sellableValue.LessThan(amountNeeded) {
		return nil, fmt.Errorf(
			"whole-share holdings can raise at most $%s of the $%s needed",
			sellableValue.StringFixed(2), amountNeeded.StringFixed(2),
		)
	}

	taxPerShare := portfolio.TaxPerShare(snapshot.Lots, request.TaxRates)
	valueAboveMin := valueAboveMinAllowed(snapshot, request.Settings.AllocationLowerLimit, amountNeeded)
	sales, err := selectLots(snapshot, lotIdxs, taxPerShare, valueAboveMin, amountNeeded)
	if err != nil {
		return nil, err
	}

	return portfolio.GroupSales(sales), nil
}
