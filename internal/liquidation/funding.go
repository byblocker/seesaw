package liquidation

import (
	"fmt"
	"seesaw/internal/utils"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

type Funding struct {
	AvailableCash decimal.Decimal
	MinTargetSale decimal.Decimal
	MaxTargetSale decimal.Decimal
}

func PlanFunding(
	balances models.Balances,
	withdrawal, reserve, maxExcess decimal.Decimal,
) (Funding, error) {
	if !withdrawal.IsPositive() {
		return Funding{}, fmt.Errorf("withdrawal must be positive")
	}
	if reserve.IsNegative() || maxExcess.IsNegative() {
		return Funding{}, fmt.Errorf("cash reserve and maximum excess cash must be nonnegative")
	}

	cashAboveReserve := utils.ClampNonNegative(balances.CashBalance.Sub(reserve))

	availableCash := utils.ClampNonNegative(decimal.Min(cashAboveReserve, balances.WithdrawableCash))
	minTargetSale := utils.ClampNonNegative(withdrawal.Add(reserve).Sub(balances.CashBalance))
	maxTargetSale := minTargetSale.Add(maxExcess)

	return Funding{
		AvailableCash: availableCash,
		MinTargetSale: minTargetSale,
		MaxTargetSale: maxTargetSale,
	}, nil
}
