package rebalancer

import (
	"context"
	"seesaw/internal/collections"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

func (r *Rebalancer) harvestAsset(
	ctx context.Context,
	asset portfolio.Asset,
	holdings []portfolio.Holding,
	recentTx models.RecentTxByTicker,
	tradeDate time.Time,
) models.SellProposal {
	primary, alternate := asset.Plan.PrimaryTicker, asset.Plan.AltTicker
	if primary == "" || alternate == "" || primary == alternate {
		return models.SellProposal{}
	}

	var sellPrimary, sellAlt models.SellProposal

	if !recentTx.HasSellWithinWashWindow(primary, tradeDate) {
		sellAlt = r.harvestPosition(ctx, alternate, holdings, recentTx, tradeDate)
	}
	if !recentTx.HasSellWithinWashWindow(alternate, tradeDate) {
		sellPrimary = r.harvestPosition(ctx, primary, holdings, recentTx, tradeDate)
	}

	if len(sellPrimary.Lots) == 0 {
		return sellAlt
	}
	if len(sellAlt.Lots) == 0 || sellPrimary.RealizedPL.LessThan(sellAlt.RealizedPL) {
		return sellPrimary
	}
	return sellAlt
}

func (r *Rebalancer) harvestPosition(
	ctx context.Context,
	ticker string,
	holdings []portfolio.Holding,
	recentTx models.RecentTxByTicker,
	tradeDate time.Time,
) models.SellProposal {
	holding, isHeld := collections.Find(slices.Values(holdings), func(holding portfolio.Holding) bool {
		return holding.Ticker == ticker
	})
	if !isHeld {
		return models.SellProposal{}
	}
	if recentTx.HasBuyWithinWashWindow(ticker, tradeDate) {
		utils.Verbosef(ctx, "Skip harvesting %s: recent purchases in the wash-sale window", ticker)
		return models.SellProposal{}
	}

	minPctLoss := *r.settings.MinPctLossToHarvest
	candidate := harvestLots(holding, minPctLoss)
	if len(candidate.Lots) == 0 {
		utils.Verbosef(ctx, "No lots of %s meet the %s%% loss threshold",
			ticker, minPctLoss.Shift(2).StringFixed(2))
	} else {
		utils.Verbosef(ctx, "Harvest candidate %s: %d lot(s), proceeds $%s, P/L $%s",
			ticker, len(candidate.Lots), candidate.Proceeds.StringFixed(2), candidate.RealizedPL.StringFixed(2))
	}
	return candidate
}

func harvestLots(holding portfolio.Holding, minPctLossToHarvest decimal.Decimal) models.SellProposal {
	proposal := models.SellProposal{
		Ticker: holding.Ticker,
		Type:   models.Market,
		Price:  holding.Bid,
	}

	for _, sale := range holding.Lots {
		gainPct := sale.GainPct()
		if !gainPct.IsNegative() || gainPct.Cmp(minPctLossToHarvest) > 0 {
			continue
		}

		proposal.Add(sale)
	}

	return proposal
}
