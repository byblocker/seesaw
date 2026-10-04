package rebalance

import (
	"seesaw/pkg/models"
)

func canBuy(recentTx models.RecentTxByTicker, ticker string) bool {
	return ticker != "" && len(recentTx[ticker].Sells) == 0
}

func SelectPosToSell(
	asset models.Asset,
	recentTx models.RecentTxByTicker,
	harvestPosition func(string) models.SellProposal,
) models.SellProposal {
	if asset.Plan == nil {
		return models.SellProposal{}
	}

	primary, alternate := asset.Plan.PrimaryTicker, asset.Plan.AltTicker
	if primary == alternate {
		return models.SellProposal{}
	}

	var sellPrimary, sellAlt models.SellProposal

	if canBuy(recentTx, primary) {
		sellAlt = harvestPosition(alternate)
	}
	if canBuy(recentTx, alternate) {
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

func CanSell(recentTx models.RecentTxByTicker, ticker string) bool {
	return ticker != "" && len(recentTx[ticker].Buys) == 0
}

func SelectPosToBuy(
	asset models.Asset,
	recentTx models.RecentTxByTicker,
	sell models.SellProposal,
	quoteByTicker map[string]models.Quote,
) BuyRequest {
	shortfall := sell.Proceeds.Sub(asset.ValueDrift)
	if !shortfall.IsPositive() {
		// We should not buy into overweight position
		return BuyRequest{}
	}

	for _, ticker := range []string{asset.Plan.PrimaryTicker, asset.Plan.AltTicker} {
		isBeingSold := len(sell.Lots) > 0 && ticker == sell.Ticker
		if !isBeingSold && canBuy(recentTx, ticker) {
			return BuyRequest{
				Ticker:     ticker,
				ValueDrift: shortfall,
				Price:      quoteByTicker[ticker].Ask,
			}
		}
	}

	return BuyRequest{}
}
