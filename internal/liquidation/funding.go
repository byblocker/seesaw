package liquidation

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func PlanFunding(balances models.Balances, request models.LiquidationRequest) *models.LiquidationPlan {
	withdrawal := request.Amount
	maxExcess := *request.Settings.MaxExcessCash

	availableCash := utils.ClampNonNegative(
		decimal.Min(balances.CashBalance, balances.WithdrawableCash),
	)
	minTargetSale := utils.ClampNonNegative(withdrawal.Sub(balances.CashBalance))
	maxTargetSale := minTargetSale.Add(maxExcess)

	return &models.LiquidationPlan{
		Request:       request,
		StartingCash:  balances.CashBalance,
		AvailableCash: availableCash,
		MinTargetSale: minTargetSale,
		MaxTargetSale: maxTargetSale,
	}
}
