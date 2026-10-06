package rebalance

import (
	"seesaw/pkg/models"
	"seesaw/pkg/orders"
	"time"

	"github.com/shopspring/decimal"
)

func HarvestTaxLoss(
	position models.Position,
	bid,
	minPctLossToHarvest decimal.Decimal,
	saleDate time.Time,
) orders.SellProposal {
	proposal := orders.SellProposal{
		Ticker: position.Ticker,
		Type:   orders.Market,
		Price:  bid,
	}

	for i := range position.Lots {
		lot := &position.Lots[i]
		if !lot.PLPct.IsNegative() || lot.PLPct.Cmp(minPctLossToHarvest) > 0 {
			continue
		}

		proposal.Add(orders.NewSellLot(position.Ticker, lot, bid, saleDate))
	}

	return proposal
}
