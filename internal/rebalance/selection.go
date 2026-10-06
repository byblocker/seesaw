package rebalance

import (
	"seesaw/pkg/models"
	"seesaw/pkg/orders"
	"seesaw/pkg/portfolio"
)

func SelectPosToSell(
	asset portfolio.Asset,
	recentTx models.RecentTxByTicker,
	harvestPosition func(string) orders.SellProposal,
) orders.SellProposal {
	if asset.Plan == nil {
		return orders.SellProposal{}
	}

	primary, alternate := asset.Plan.PrimaryTicker, asset.Plan.AltTicker
	if primary == "" || alternate == "" || primary == alternate {
		return orders.SellProposal{}
	}

	var sellPrimary, sellAlt orders.SellProposal

	if !recentTx.HasRecentSell(primary) {
		sellAlt = harvestPosition(alternate)
	}
	if !recentTx.HasRecentSell(alternate) {
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
	sell orders.SellProposal,
	quoteByTicker map[string]models.Quote,
) BuyRequest {
	shortfall := sell.Proceeds.Sub(asset.ValueDrift)
	if !shortfall.IsPositive() {
		// We should not buy into overweight position
		return BuyRequest{}
	}

	for ticker := range asset.Plan.Tickers() {
		isBeingSold := len(sell.Lots) > 0 && ticker == sell.Ticker
		if !isBeingSold && !recentTx.HasRecentSell(ticker) {
			return BuyRequest{
				Ticker:     ticker,
				ValueDrift: shortfall,
				Price:      quoteByTicker[ticker].Ask,
			}
		}
	}

	return BuyRequest{}
}
