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
		proposal.PositionIdx = lot.PositionIdx
		proposal.Add(sale, lot)
	}
	return slices.DeleteFunc(proposals, SellProposal.IsEmpty)
}

// Add a sale of lot at its bid.
func (proposal *SellProposal) Add(sale LotSale, lot Lot) {
	proposal.Lots = append(proposal.Lots, sale)
	proposal.Proceeds = proposal.Proceeds.Add(sale.Quantity.Mul(lot.Snapshot.Bid))
	proposal.RealizedPL = proposal.RealizedPL.Add(sale.Quantity.Mul(lot.Snapshot.GainPerShare))
}

func (proposal SellProposal) IsEmpty() bool {
	return len(proposal.Lots) == 0
}

type LotSale struct {
	LotIdx   int
	Quantity decimal.Decimal
}
