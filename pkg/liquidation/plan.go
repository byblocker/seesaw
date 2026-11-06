package liquidation

import (
	"fmt"
	"log"

	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

func PlanSales(
	snapshot portfolio.Snapshot,
	settings config.Settings,
	amount decimal.Decimal,
) (portfolio.Proposal, error) {
	// Use existing cash first, and sell only for the rest of the withdrawal.
	amountNeeded := utils.ClampNonNegative(amount.Sub(snapshot.Balances.CashBalance))
	if amountNeeded.IsZero() {
		log.Print("Liquidation planning complete: 0 sell(s)")
		return portfolio.Proposal{Withdrawal: amount}, nil
	}

	log.Printf("Selecting liquidation sales for at least %s", utils.FormatMoney(amountNeeded))
	lotIdxs := sellableLots(snapshot)

	sellableValue := decimal.Zero
	for _, lotIdx := range lotIdxs {
		lot := snapshot.Lots[lotIdx]
		sellableValue = sellableValue.Add(lot.Snapshot.Bid.Mul(decimal.NewFromInt(lot.WholeShares())))
	}
	if sellableValue.LessThan(amountNeeded) {
		return portfolio.Proposal{}, fmt.Errorf(
			"whole-share holdings can raise at most $%s of the $%s needed",
			sellableValue.StringFixed(2), amountNeeded.StringFixed(2),
		)
	}

	valueAboveMin := valueAboveMinAllowed(snapshot, settings.AllocationLowerLimit, amountNeeded)
	sales, err := selectLots(snapshot, lotIdxs, valueAboveMin, amountNeeded)
	if err != nil {
		return portfolio.Proposal{}, err
	}

	sells := portfolio.GroupSales(snapshot, sales)
	log.Printf("Liquidation planning complete: %d sell(s)", len(sells))
	return portfolio.Proposal{Sells: sells, Withdrawal: amount}, nil
}
