package liquidation

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func PlanFunding(balances models.Balances, request models.LiquidationRequest) *models.LiquidationPlan {
	withdrawal := request.Amount
	reserve := *request.Settings.CashReserve
	maxExcess := *request.Settings.MaxExcessCash

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
	}
}
