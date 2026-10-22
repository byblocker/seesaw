package liquidation

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"
)

func PlanFunding(balances models.Balances, request models.LiquidationRequest) *models.LiquidationPlan {
	withdrawal := request.Amount
	maxExcess := request.Settings.MaxExcessCash

	minTargetSale := utils.ClampNonNegative(withdrawal.Sub(balances.CashBalance))
	maxTargetSale := minTargetSale.Add(maxExcess)

	return &models.LiquidationPlan{
		Request:       request,
		StartingCash:  balances.CashBalance,
		MinTargetSale: minTargetSale,
		MaxTargetSale: maxTargetSale,
	}
}
