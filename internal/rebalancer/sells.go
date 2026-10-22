package rebalancer

import (
	"context"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"time"

	"github.com/shopspring/decimal"
)

func (r *Rebalancer) harvestAsset(
	ctx context.Context,
	asset portfolio.Asset,
	tradeDate time.Time,
) models.SellProposal {
	// A planned asset's positions follow its tickers: the primary, then any alternate.
	if len(asset.Positions) < 2 || asset.Positions[0].Ticker == asset.Positions[1].Ticker {
		return models.SellProposal{}
	}
	primary, alternate := asset.Positions[0], asset.Positions[1]

	var sellPrimary, sellAlt models.SellProposal

	if !primary.RecentlySold {
		sellAlt = r.harvestPosition(ctx, alternate, tradeDate)
	}
	if !alternate.RecentlySold {
		sellPrimary = r.harvestPosition(ctx, primary, tradeDate)
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
	position portfolio.Position,
	tradeDate time.Time,
) models.SellProposal {
	ticker := position.Ticker
	if !position.IsHeld() {
		return models.SellProposal{}
	}
	if position.RecentlyBought {
		utils.Verbosef(ctx, "Skip harvesting %s: recent purchases in the wash-sale window", ticker)
		return models.SellProposal{}
	}

	minPctLoss := r.settings.MinPctLossToHarvest
	candidate := harvestLots(position, tradeDate, minPctLoss)
	if len(candidate.Lots) == 0 {
		utils.Verbosef(ctx, "No lots of %s meet the %s%% loss threshold",
			ticker, minPctLoss.Shift(2).StringFixed(2))
	} else {
		utils.Verbosef(ctx, "Harvest candidate %s: %d lot(s), proceeds $%s, P/L $%s",
			ticker, len(candidate.Lots), candidate.Proceeds.StringFixed(2), candidate.RealizedPL.StringFixed(2))
	}
	return candidate
}

func harvestLots(position portfolio.Position, saleDate time.Time, minPctLossToHarvest decimal.Decimal) models.SellProposal {
	proposal := models.SellProposal{
		Ticker: position.Ticker,
		Type:   models.Market,
		Price:  position.Quote.Bid,
	}

	for _, sale := range position.LotSales(saleDate) {
		gainPct := sale.GainPct()
		if !gainPct.IsNegative() || gainPct.Cmp(minPctLossToHarvest) > 0 {
			continue
		}

		proposal.Add(sale)
	}

	return proposal
}
