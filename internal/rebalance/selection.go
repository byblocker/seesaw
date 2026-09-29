package rebalance

import (
	"seesaw/pkg/models"
)

func canBuy(asset models.Asset, ticker string) bool {
	return ticker != "" && len(asset.RecentTx[ticker].Sells) == 0
}

func SelectPosToSell(
	asset models.Asset,
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

func CanSell(asset models.Asset, ticker string) bool {
	return ticker != "" && len(asset.RecentTx[ticker].Buys) == 0
}

func SelectPosToBuy(
	asset models.Asset,
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
		if !isBeingSold && canBuy(asset, ticker) {
			return BuyRequest{
				Ticker:     ticker,
				ValueDrift: shortfall,
				Price:      quoteByTicker[ticker].Ask,
			}
		}
	}

	return BuyRequest{}
}
