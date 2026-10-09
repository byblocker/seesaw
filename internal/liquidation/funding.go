package liquidation

import (
	"fmt"
	"seesaw/internal/utils"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func PlanFunding(
	balances models.Balances,
	request models.LiquidationRequest,
) (*models.LiquidationPlan, error) {
	if err := request.Settings.Validate(); err != nil {
		return nil, err
	}

	withdrawal := request.Amount
	if !withdrawal.IsPositive() {
		return nil, fmt.Errorf("withdrawal must be positive")
	}

	reserve := *request.Settings.CashReserve
	maxExcess := *request.Settings.MaxExcessCash
	if reserve.IsNegative() || maxExcess.IsNegative() {
		return nil, fmt.Errorf("cash reserve and maximum excess cash must be non-negative")
	}

	cashAboveReserve := utils.ClampNonNegative(balances.CashBalance.Sub(reserve))

	availableCash := utils.ClampNonNegative(
		decimal.Min(cashAboveReserve, balances.WithdrawableCash),
	)
	minTargetSale := utils.ClampNonNegative(
		withdrawal.Add(reserve).Sub(balances.CashBalance),
	)
	maxTargetSale := minTargetSale.Add(maxExcess)

	return &models.LiquidationPlan{
		Request:       request,
		StartingCash:  balances.CashBalance,
		AvailableCash: availableCash,
		MinTargetSale: minTargetSale,
		MaxTargetSale: maxTargetSale,
	}, nil
}
