package rebalance

import (
	"seesaw/pkg/models"
	"time"

	"github.com/shopspring/decimal"
)

func HarvestTaxLoss(
	position models.Position,
	bid,
	minPctLossToHarvest decimal.Decimal,
	saleDate time.Time,
) models.SellProposal {
	proposal := models.SellProposal{
		Ticker: position.Ticker,
		Type:   models.Market,
		Price:  bid,
	}

	for i := range position.Lots {
		lot := &position.Lots[i]
		if !lot.PLPct.IsNegative() || lot.PLPct.Cmp(minPctLossToHarvest) > 0 {
			continue
		}

		proposal.Add(models.NewSellLot(position.Ticker, lot, bid, saleDate))
	}

	return proposal
}
