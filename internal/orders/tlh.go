package orders

import (
	"seesaw/internal/config"
	"seesaw/pkg/etrade"
	"sort"

	"github.com/shopspring/decimal"
)

type SellProposal struct {
	Ticker     string
	Type       OrderType
	Price      decimal.Decimal
	Lots       []SellLot
	Proceeds   decimal.Decimal
	RealizedPL decimal.Decimal
}

func (p *SellProposal) Add(sell SellLot) {
	p.Lots = append(p.Lots, sell)
	p.Proceeds = p.Proceeds.Add(sell.Quantity.Mul(p.Price))
	p.RealizedPL = p.RealizedPL.Add(sell.Quantity.Mul(p.Price.Sub(sell.Lot.PricePaid)))
}

type SellLot struct {
	Lot      etrade.PositionLot
	Quantity decimal.Decimal
}

func HarvestTaxLoss(
	position etrade.Position,
	bid decimal.Decimal,
) SellProposal {
	proposal := SellProposal{
		Ticker: position.Ticker,
		Type:   Market,
		Price:  bid,
	}

	sort.Slice(position.Lots, func(i, j int) bool {
		return position.Lots[i].PLPct.LessThan(position.Lots[j].PLPct)
	})

	for _, lot := range position.Lots {
		if lot.PLPct.Cmp(config.MinPctLossToHarvest) > 0 {
			break
		}

		proposal.Add(SellLot{Quantity: lot.Quantity, Lot: lot})
	}

	return proposal
}
