package portfolio

import (
	"slices"

	"github.com/shopspring/decimal"
)

type Buy struct {
	PositionIdx   int
	Price         decimal.Decimal
	Quantity      decimal.Decimal
	EstimatedCost decimal.Decimal
}

type SellProposal struct {
	PositionIdx int
	Price       decimal.Decimal
	Lots        []LotSale
	Proceeds    decimal.Decimal
	RealizedPL  decimal.Decimal
}

// One proposal per position sold, in position order.
func GroupSales(snapshot Snapshot, sales []LotSale) []SellProposal {
	proposals := make([]SellProposal, len(snapshot.Positions)) // by position row
	for _, sale := range sales {
		lot := snapshot.Lots[sale.LotIdx]
		proposal := &proposals[lot.PositionIdx]
		if proposal.IsEmpty() {
			*proposal = SellProposal{PositionIdx: lot.PositionIdx, Price: lot.Market.Bid}
		}
		proposal.Add(sale, lot)
	}
	return slices.DeleteFunc(proposals, SellProposal.IsEmpty)
}

// Add a sale of lot at the proposal's price.
func (p *SellProposal) Add(sale LotSale, lot Lot) {
	p.Lots = append(p.Lots, sale)
	p.Proceeds = p.Proceeds.Add(sale.Quantity.Mul(p.Price))
	p.RealizedPL = p.RealizedPL.Add(sale.Quantity.Mul(lot.Market.GainPerShare))
}

func (p SellProposal) IsEmpty() bool {
	return len(p.Lots) == 0
}

type LotSale struct {
	LotIdx   int
	Quantity decimal.Decimal
}
