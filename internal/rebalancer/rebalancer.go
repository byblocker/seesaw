package rebalancer

import (
	"context"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"time"
)

type Rebalancer struct {
	settings config.RebalanceSettings
}

func New(settings config.RebalanceSettings) *Rebalancer {
	return &Rebalancer{settings: settings}
}

func (r *Rebalancer) RebalanceAsset(
	ctx context.Context,
	asset portfolio.Asset,
	holdings []portfolio.Holding,
	quotes map[string]models.Quote,
	recentTx models.RecentTxByTicker,
	tradeDate time.Time,
) ([]models.SellProposal, []BuyRequest) {
	if asset.Plan == nil {
		utils.Verbosef(ctx, "Evaluating holdings outside the portfolio plan for tax losses")
		return r.harvestUnplanned(ctx, asset, holdings, recentTx, tradeDate), nil
	}

	utils.Verbosef(ctx, "Evaluating %s: current %s%%, target %s%%",
		asset.Label(),
		asset.Allocation.Shift(2).StringFixed(2), asset.Plan.TargetAllocation.Shift(2).StringFixed(2))
	for ticker := range asset.Plan.Tickers() {
		if recentTx.HasSellWithinWashWindow(ticker, tradeDate) {
			utils.Verbosef(ctx, "Skip buying %s: recent sales in the wash-sale window", ticker)
		}
	}

	sell := r.harvestAsset(ctx, asset, holdings, recentTx, tradeDate)
	buy, ok := selectBuy(asset, recentTx, tradeDate, sell, quotes)
	if !ok {
		// This should rarely happen, but if we cannot maintain an asset's allocation
		// by purchasing its paired ticker, we should not harvest a ticker
		if !sell.Proceeds.Sub(asset.ValueDrift).IsPositive() {
			utils.Verbosef(ctx, "Skip %s: no allocation shortfall after considering sales", asset.Plan.PrimaryTicker)
		} else {
			utils.Verbosef(ctx, "Skip %s: no eligible purchase ticker", asset.Plan.PrimaryTicker)
		}
		return nil, nil
	}

	utils.Verbosef(ctx, "Queue purchase of %s to cover a $%s shortfall", buy.Ticker, buy.Amount.StringFixed(2))
	if len(sell.Lots) == 0 {
		return nil, []BuyRequest{buy}
	}
	return []models.SellProposal{sell}, []BuyRequest{buy}
}

// We should not force liquidation of unplanned positions while rebalancing,
// but we should eagerly harvest any tax-loss these positions have accrued.
func (r *Rebalancer) harvestUnplanned(
	ctx context.Context,
	asset portfolio.Asset,
	holdings []portfolio.Holding,
	recentTx models.RecentTxByTicker,
	tradeDate time.Time,
) []models.SellProposal {
	var sells []models.SellProposal
	for _, ticker := range asset.Tickers {
		if sell := r.harvestPosition(ctx, ticker, holdings, recentTx, tradeDate); len(sell.Lots) > 0 {
			sells = append(sells, sell)
		}
	}
	return sells
}
