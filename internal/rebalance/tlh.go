package rebalance

import (
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

func HarvestTaxLoss(holding portfolio.Holding, minPctLossToHarvest decimal.Decimal) models.SellProposal {
	proposal := models.SellProposal{
		Ticker: holding.Ticker,
		Type:   models.Market,
		Price:  holding.Bid,
	}

	for _, sale := range holding.Lots {
		lossPct := sale.Lot.PLPct
		if !lossPct.IsNegative() || lossPct.Cmp(minPctLossToHarvest) > 0 {
			continue
		}

		proposal.Add(sale)
	}

	return proposal
}
