package rebalance

import (
	"seesaw/pkg/models"
	"time"

	"github.com/shopspring/decimal"
)

// Number of days before a future transaction no longer affects rebalancing
const WashSaleWindowDays = 31

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

		proposal.Add(models.SellLot{
			SaleCandidate: models.NewSaleCandidate(position.Ticker, lot, bid, saleDate),
			Quantity:      lot.Quantity,
		})
	}

	return proposal
}
