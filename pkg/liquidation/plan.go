package liquidation

import (
	"fmt"
	"log"

	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type Request struct {
	Amount   decimal.Decimal
	Settings config.Settings
	TaxRates *config.TaxSettings // Nil for non-taxable accounts
}

func (request Request) Validate() error {
	if !request.Amount.IsPositive() {
		return fmt.Errorf("withdrawal must be positive")
	}
	if err := request.Settings.Validate(); err != nil {
		return err
	}
	if request.TaxRates != nil {
		return request.TaxRates.Validate()
	}
	return nil
}

func PlanSales(
	snapshot portfolio.Snapshot,
	request Request,
) ([]portfolio.SellProposal, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}

	// Use existing cash first, and sell only for the rest of the withdrawal.
	amountNeeded := utils.ClampNonNegative(request.Amount.Sub(snapshot.Balances.CashBalance))
	if amountNeeded.IsZero() {
		log.Print("Liquidation planning complete: 0 sell(s)")
		return nil, nil
	}

	log.Printf("Selecting liquidation sales for at least %s", utils.FormatMoney(amountNeeded))
	lotIdxs := sellableLots(snapshot)

	sellableValue := decimal.Zero
	for _, lotIdx := range lotIdxs {
		lot := snapshot.Lots[lotIdx]
		sellableValue = sellableValue.Add(lot.Snapshot.Bid.Mul(decimal.NewFromInt(lot.WholeShares())))
	}
	if sellableValue.LessThan(amountNeeded) {
		return nil, fmt.Errorf(
			"whole-share holdings can raise at most $%s of the $%s needed",
			sellableValue.StringFixed(2), amountNeeded.StringFixed(2),
		)
	}

	taxPerDollar := portfolio.TaxPerDollar(snapshot.Lots, request.TaxRates)
	valueAboveMin := valueAboveMinAllowed(snapshot, request.Settings.AllocationLowerLimit, amountNeeded)
	sales, err := selectLots(snapshot, lotIdxs, taxPerDollar, valueAboveMin, amountNeeded)
	if err != nil {
		return nil, err
	}

	sells := portfolio.GroupSales(snapshot, sales)
	log.Printf("Liquidation planning complete: %d sell(s)", len(sells))
	return sells, nil
}
