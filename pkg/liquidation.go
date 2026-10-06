package pkg

import (
	"context"
	"fmt"
	"log"
	"seesaw/internal/liquidation"
	"seesaw/pkg/models"
	"time"

	"github.com/shopspring/decimal"
)

type LiquidationPlan struct {
	Sells         []models.SellProposal
	Withdrawal    decimal.Decimal
	AvailableCash decimal.Decimal
	TotalProceeds decimal.Decimal
}

func Liquidate(
	ctx context.Context,
	snapshot models.AccountSnapshot,
	client models.BrokerClient,
	settings models.LiquidationSettings,
	withdrawal decimal.Decimal,
	rates *models.TaxSettings,
	searchLimit int,
) (*LiquidationPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	if snapshot.Portfolio == nil {
		return nil, fmt.Errorf("snapshot is missing portfolio")
	}
	if rates != nil {
		if err := rates.Validate(); err != nil {
			return nil, err
		}
	}
	funding, err := liquidation.PlanFunding(
		snapshot.Portfolio.Balances,
		withdrawal,
		*settings.CashReserve,
		*settings.MaxExcessCash,
	)
	if err != nil {
		return nil, err
	}
	plan := &LiquidationPlan{
		Withdrawal:    withdrawal,
		AvailableCash: funding.AvailableCash,
		TotalProceeds: snapshot.Portfolio.Balances.CashBalance.Sub(withdrawal),
	}
	if funding.MinTargetSale.IsZero() {
		return plan, nil
	}
	if searchLimit <= 0 {
		return nil, fmt.Errorf("search limit must be positive")
	}

	saleDate := time.Now()
	var tickers []string
	for _, asset := range snapshot.Assets {
		for _, ticker := range asset.Tickers {
			if snapshot.Portfolio.PositionByTicker[ticker].Quantity.IsPositive() {
				tickers = append(tickers, ticker)
			}
		}
	}
	quotes, err := fetchQuotes(ctx, client, tickers)
	if err != nil {
		return nil, err
	}

	candidates, err := liquidation.RankLots(snapshot, quotes, settings.LotSelection, rates, saleDate)
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
		*settings.MaxAllocationDeviation,
		settings.LotSelection,
		rates,
		searchLimit,
	)
	if err != nil {
		return nil, err
	}

	for _, sale := range plan.Sells {
		plan.TotalProceeds = plan.TotalProceeds.Add(sale.Proceeds)
	}

	log.Printf("Liquidation planning complete: %d sell(s)", len(plan.Sells))
	return plan, nil
}
