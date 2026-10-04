package rebalance

import (
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

// Number of days before a future transaction no longer affects rebalancing
const WashSaleWindowDays = 31

func HarvestTaxLoss(
	position models.Position,
	bid,
	minPctLossToHarvest decimal.Decimal,
) models.SellProposal {
	proposal := models.SellProposal{
		Ticker: position.Ticker,
		Type:   models.Market,
		Price:  bid,
	}

	for _, lot := range position.Lots {
		if !lot.PLPct.IsNegative() || lot.PLPct.Cmp(minPctLossToHarvest) > 0 {
			continue
		}

		proposal.Add(models.SellLot{Quantity: lot.Quantity, Lot: lot})
	}

	return proposal
}
