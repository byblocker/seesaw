package pkg

import (
	"context"
	"fmt"
	"seesaw/internal/config"
	"seesaw/internal/orders"
	"seesaw/pkg/etrade"

	"github.com/shopspring/decimal"
)

type RebalancePlan struct {
	Buys  []orders.Buy
	Sells []orders.SellProposal
}

func Rebalance(
	ctx context.Context,
	snapshot AccountSnapshot,
	client *etrade.Client,
) (*RebalancePlan, error) {
	if snapshot.TotalValue.Cmp(decimal.Zero) <= 0 {
		return nil, fmt.Errorf("empty or negative portfolio")
	}

	var (
		buyRequests []orders.BuyRequest
		plan        RebalancePlan
	)

	balance := snapshot.BuyingPower
	maxValueDeviationAllowed := config.MaxPctAllocDeviationAllowed.Mul(snapshot.TotalValue)

	quotePerTicker := make(map[string]etrade.Quote, len(snapshot.Tickers))
	for _, ticker := range snapshot.Tickers {
		quote, err := client.Quote(ctx, ticker)
		if err != nil {
			return nil, err
		}
		quotePerTicker[ticker] = quote
	}

	for _, asset := range snapshot.Assets {
		harvestFunc := func(ticker string) orders.SellProposal {
			position, exists := asset.Positions[ticker]
			if !exists || !canSell(asset, ticker) {
				return orders.SellProposal{}
			}

			return orders.HarvestTaxLoss(position, quotePerTicker[ticker].Bid)
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

		sell := selectPosToSell(asset, harvestFunc)
		buy := selectPosToBuy(asset, sell, quotePerTicker)

		if buy.Ticker == config.NoTicker {
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

		buy, err := orders.ProposeBuy(request, balance, maxValueDeviationAllowed)
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

func canBuy(asset Asset, ticker string) bool {
	return ticker != "" && len(asset.RecentTx[ticker].Sells) == 0
}

func selectPosToSell(
	asset Asset,
	harvestPosition func(string) orders.SellProposal,
) orders.SellProposal {
	if asset.Plan == nil {
		return orders.SellProposal{}
	}

	primary, alternate := asset.Plan.PrimaryTicker, asset.Plan.AltTicker
	if primary == alternate {
		return orders.SellProposal{}
	}

	var sellPrimary, sellAlt orders.SellProposal

	if canBuy(asset, primary) {
		sellAlt = harvestPosition(alternate)
	}
	if canBuy(asset, alternate) {
		sellPrimary = harvestPosition(primary)
	}

	if len(sellPrimary.Lots) == 0 {
		return sellAlt
	}
	if len(sellAlt.Lots) == 0 || sellPrimary.RealizedPL.LessThan(sellAlt.RealizedPL) {
		return sellPrimary
	}
	return sellAlt
}

func canSell(asset Asset, ticker string) bool {
	return ticker != "" && len(asset.RecentTx[ticker].Buys) == 0
}

func selectPosToBuy(
	asset Asset,
	sell orders.SellProposal,
	quoteByTicker map[string]etrade.Quote,
) orders.BuyRequest {
	shortfall := sell.Proceeds.Sub(asset.ValueDrift)
	if !shortfall.IsPositive() {
		// We should not buy into overweight position
		return orders.BuyRequest{}
	}

	for _, ticker := range []string{asset.Plan.PrimaryTicker, asset.Plan.AltTicker} {
		isBeingSold := len(sell.Lots) > 0 && ticker == sell.Ticker
		if !isBeingSold && canBuy(asset, ticker) {
			return orders.BuyRequest{
				Ticker:     ticker,
				ValueDrift: shortfall,
				Price:      quoteByTicker[ticker].Ask,
			}
		}
	}

	return orders.BuyRequest{}
}
