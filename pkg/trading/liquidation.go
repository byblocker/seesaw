package trading

import (
	"context"
	"fmt"
	"log"
	"seesaw/internal/liquidation"
	"seesaw/pkg/models"
	"seesaw/pkg/orders"
	"seesaw/pkg/portfolio"
	"time"

	"github.com/shopspring/decimal"
)

type LiquidationPlan struct {
	Sells           []orders.SellProposal
	RequestAmount   decimal.Decimal
	AvailableCash   decimal.Decimal
	ExpectedBalance decimal.Decimal
}

func Liquidate(
	ctx context.Context,
	snapshot portfolio.Snapshot,
	client models.BrokerClient,
	request models.LiquidationRequest,
) (*LiquidationPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := request.Settings.Validate(); err != nil {
		return nil, err
	}
	if snapshot.Portfolio == nil {
		return nil, fmt.Errorf("snapshot is missing portfolio")
	}
	if request.TaxRates != nil {
		if err := request.TaxRates.Validate(); err != nil {
			return nil, err
		}
	}

	balances := snapshot.Portfolio.Balances

	funding, err := liquidation.PlanFunding(
		balances,
		request.Amount,
		*request.Settings.CashReserve,
		*request.Settings.MaxExcessCash,
	)
	if err != nil {
		return nil, err
	}

	plan := &LiquidationPlan{
		RequestAmount:   request.Amount,
		AvailableCash:   funding.AvailableCash,
		ExpectedBalance: balances.CashBalance.Sub(request.Amount),
	}
	if funding.MinTargetSale.IsZero() {
		return plan, nil
	}

	if request.SearchLimit <= 0 {
		return nil, fmt.Errorf("search limit must be positive")
	}

	saleDate := time.Now()
	quotes, err := client.QuoteAll(ctx, snapshot.Portfolio.ActiveTickers())
	if err != nil {
		return nil, err
	}

	candidates, err := liquidation.CollectLots(snapshot, quotes, saleDate)
	if err != nil {
		return nil, err
	}

	log.Printf(
		"Selecting liquidation sales between $%s and $%s",
		funding.MinTargetSale.StringFixed(2), funding.MaxTargetSale.StringFixed(2),
	)

	plan.Sells, err = liquidation.SelectWholeShareSales(
		ctx,
		candidates,
		funding,
		request,
	)
	if err != nil {
		return nil, err
	}

	for _, sale := range plan.Sells {
		plan.ExpectedBalance = plan.ExpectedBalance.Add(sale.Proceeds)
	}

	log.Printf("Liquidation planning complete: %d sell(s)", len(plan.Sells))
	return plan, nil
}
