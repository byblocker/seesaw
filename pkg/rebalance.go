package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/rebalance"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

type RebalancePlan struct {
	Buys  []models.Buy
	Sells []models.SellProposal
}

func Rebalance(
	ctx context.Context,
	snapshot models.AccountSnapshot,
	client models.Broker,
	settings models.RebalanceSettings,
) (*RebalancePlan, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	if snapshot.TotalValue.Cmp(decimal.Zero) <= 0 {
		return nil, fmt.Errorf("empty or negative portfolio")
	}

	var (
		buyRequests []rebalance.BuyRequest
		plan        RebalancePlan
	)

	brokerSettings := client.Settings()

	balance := snapshot.BuyingPower
	maxValueDeviationAllowed := snapshot.TotalValue.Mul(
		*settings.MaxPctAllocDeviationAllowed,
	)

	quotePerTicker := make(map[string]models.Quote, len(snapshot.Tickers))
	for _, ticker := range snapshot.Tickers {
		quote, err := client.Quote(ctx, ticker)
		if err != nil {
			return nil, err
		}
		quotePerTicker[ticker] = quote
	}

	for _, asset := range snapshot.Assets {
		harvestFunc := func(ticker string) models.SellProposal {
			position, exists := asset.Positions[ticker]
			if !exists || !rebalance.CanSell(asset, ticker) {
				return models.SellProposal{}
			}

			return rebalance.HarvestTaxLoss(
				position,
				quotePerTicker[ticker].Bid,
				*settings.MinPctLossToHarvest,
			)
		}

		if asset.Plan == nil {
			// We should not force liquidation of unplanned positions while rebalancing,
			// but we should eagerly harvest any tax-loss these positions have accrued.
			for ticker := range asset.Positions {

				if sell := harvestFunc(ticker); len(sell.Lots) > 0 {
					plan.Sells = append(plan.Sells, sell)
					balance = balance.Add(sell.Proceeds)
				}
			}
			continue
		}

		sell := rebalance.SelectPosToSell(asset, harvestFunc)
		buy := rebalance.SelectPosToBuy(asset, sell, quotePerTicker)

		if buy.Ticker == "" {
			// This should rarely happen, but if we cannot maintain an asset's allocation
			// by purchasing its paired ticker, we should not harvest a ticker
			continue
		}

		buyRequests = append(buyRequests, buy)

		if len(sell.Lots) > 0 {
			plan.Sells = append(plan.Sells, sell)
			balance = balance.Add(sell.Proceeds)
		}
	}

	for _, request := range buyRequests {
		if balance.Cmp(decimal.Zero) <= 0 {
			break
		}

		buy, err := rebalance.ProposeBuy(
			request,
			balance,
			maxValueDeviationAllowed,
			*settings.MinFractionalShareToRound,
			brokerSettings.AllowFractionalShares,
		)
		if err != nil {
			return nil, fmt.Errorf("size buy for %s: %w", request.Ticker, err)
		}

		if buy.Quantity.IsZero() {
			continue
		}

		plan.Buys = append(plan.Buys, buy)
		balance = balance.Sub(buy.EstimatedCost)
	}

	return &plan, nil
}
