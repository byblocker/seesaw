package portfolio

import (
	"cmp"
	"slices"

	"github.com/shopspring/decimal"
)

// The orders a planner proposes, and the cash it plans to withdraw.
type Proposal struct {
	Sells      []SellProposal
	Buys       []Buy
	Withdrawal decimal.Decimal
}

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

func (lot Lot) WholeShares() int64 {
	return lot.Quantity.Floor().IntPart()
}

// Whole shares at price that raise at most amount.
func SharesAtMost(amount, price decimal.Decimal) int64 {
	if !amount.IsPositive() {
		return 0
	}

	shares, _ := amount.QuoRem(price, 0)
	return shares.IntPart()
}

type LotSale struct {
	LotIdx   int
	Quantity decimal.Decimal
}

// Order lots to sell: holdings outside the plan first, then the lowest tax per dollar raised,
// then the most overweight asset. Remaining ties keep the given order, so plans repeat exactly.
func (snapshot Snapshot) SortLotsForSale(lotIdxs []int) {
	slices.SortStableFunc(lotIdxs, func(leftIdx, rightIdx int) int {
		left, right := snapshot.Lots[leftIdx], snapshot.Lots[rightIdx]
		leftAsset, rightAsset := snapshot.Assets[left.AssetIdx], snapshot.Assets[right.AssetIdx]
		return cmp.Or(
			cmp.Compare(preferUnplanned(leftAsset), preferUnplanned(rightAsset)),
			left.Snapshot.TaxPerDollar.Cmp(right.Snapshot.TaxPerDollar),
			rightAsset.AllocDrift.Cmp(leftAsset.AllocDrift),
		)
	})
}

func preferUnplanned(asset Asset) int {
	if asset.Plan == nil {
		return 0
	}
	return 1
}
