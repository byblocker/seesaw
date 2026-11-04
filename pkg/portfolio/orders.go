package portfolio

import "github.com/shopspring/decimal"

type RebalancePlan struct {
	Buys  []Buy
	Sells []SellProposal
}

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

// One proposal per position sold, in the order of each position's first sale.
func GroupSales(snapshot Snapshot, sales []LotSale) []SellProposal {
	var proposals []SellProposal
	proposalByPosition := map[int]int{}

	for _, sale := range sales {
		lot := snapshot.Lots[sale.LotIdx]
		i, exists := proposalByPosition[lot.PositionIdx]
		if !exists {
			i = len(proposals)
			proposalByPosition[lot.PositionIdx] = i
			proposals = append(proposals, SellProposal{PositionIdx: lot.PositionIdx, Price: snapshot.Positions[lot.PositionIdx].Quote.Bid})
		}

		proposals[i].Add(sale, lot)
	}

	return proposals
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
