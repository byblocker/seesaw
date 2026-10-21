package rebalancer

import (
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"time"
)

func SelectPosToSell(
	asset portfolio.Asset,
	recentTx models.RecentTxByTicker,
	tradeDate time.Time,
	harvestPosition func(string) models.SellProposal,
) models.SellProposal {
	if asset.Plan == nil {
		return models.SellProposal{}
	}

	primary, alternate := asset.Plan.PrimaryTicker, asset.Plan.AltTicker
	if primary == "" || alternate == "" || primary == alternate {
		return models.SellProposal{}
	}

	var sellPrimary, sellAlt models.SellProposal

	if !recentTx.HasSellWithinWashWindow(primary, tradeDate) {
		sellAlt = harvestPosition(alternate)
	}
	if !recentTx.HasSellWithinWashWindow(alternate, tradeDate) {
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

func SelectPosToBuy(
	asset portfolio.Asset,
	recentTx models.RecentTxByTicker,
	tradeDate time.Time,
	sell models.SellProposal,
	quoteByTicker map[string]models.Quote,
) BuyRequest {
	shortfall := sell.Proceeds.Sub(asset.ValueDrift)
	if !shortfall.IsPositive() {
		// We should not buy into overweight position
		return BuyRequest{}
	}

	for ticker := range asset.Plan.Tickers() {
		isBeingSold := len(sell.Lots) > 0 && ticker == sell.Ticker
		if !isBeingSold && !recentTx.HasSellWithinWashWindow(ticker, tradeDate) {
			return BuyRequest{
				Ticker: ticker,
				Amount: shortfall,
				Ask:    quoteByTicker[ticker].Ask,
			}
		}
	}

	return BuyRequest{}
}
