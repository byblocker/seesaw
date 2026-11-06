package liquidation

import (
	"fmt"
	"log"

	"seesaw/internal/format"
	"seesaw/pkg/config"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

func PlanSales(
	snapshot portfolio.Snapshot,
	settings config.Settings,
	amount decimal.Decimal,
) (portfolio.Proposal, error) {
	if !amount.IsPositive() {
		return portfolio.Proposal{}, fmt.Errorf("withdrawal must be positive")
	}
	if err := settings.Validate(); err != nil {
		return portfolio.Proposal{}, err
	}

	// Use existing cash first, and sell only for the rest of the withdrawal.
	amountNeeded := decimal.Max(decimal.Zero, amount.Sub(snapshot.Balances.CashBalance))
	if amountNeeded.IsZero() {
		log.Print("Liquidation planning complete: 0 sell(s)")
		return portfolio.Proposal{Withdrawal: amount}, nil
	}

	log.Printf("Selecting liquidation sales for at least %s", format.Money(amountNeeded))
	lotIdxs := sellableLots(snapshot)

	sellableValue := decimal.Zero
	for _, lotIdx := range lotIdxs {
		lot := snapshot.Lots[lotIdx]
		sellableValue = sellableValue.Add(lot.Snapshot.Bid.Mul(decimal.NewFromInt(lot.WholeShares())))
	}
	if sellableValue.LessThan(amountNeeded) {
		return portfolio.Proposal{}, fmt.Errorf(
			"whole-share holdings can raise at most %s of the %s needed",
			format.Money(sellableValue), format.Money(amountNeeded),
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
