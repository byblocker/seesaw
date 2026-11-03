package portfolio

import (
	"cmp"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

type LotSnapshot struct {
	models.Lot
	PositionIdx int

	// Selling at the snapshot's bid on its date
	GainPerShare decimal.Decimal
	GainPct      decimal.Decimal // Fraction of the price paid
	LongTerm     bool
	IsWashSale   bool
}

func newLotSnapshot(lot models.Lot, bid decimal.Decimal, createdAt time.Time, recentlyBought bool) LotSnapshot {
	gainPerShare := bid.Sub(lot.PricePaid)
	gainPct := decimal.Zero
	if lot.PricePaid.IsPositive() {
		gainPct, _ = gainPerShare.QuoRem(lot.PricePaid, utils.DivisionDecimalPlaces)
	}

	return LotSnapshot{
		Lot:          lot,
		GainPerShare: gainPerShare,
		GainPct:      gainPct,
		LongTerm:     isLongTerm(lot.TimeAcquired, createdAt),
		IsWashSale:   recentlyBought && gainPerShare.IsNegative(),
	}
}

// A lot row for every held lot, recorded in its position's LotIdxs in lot ID order.
func newLotSnapshots(positions []Position, positionIdxByTicker map[string]int, heldLots []models.Lot, createdAt time.Time) []LotSnapshot {
	var lots []LotSnapshot
	for _, lot := range heldLots {
		if !lot.Quantity.IsPositive() {
			continue
		}
		positionIdx := positionIdxByTicker[lot.Ticker] // newAssets gave every held ticker a position
		position := &positions[positionIdx]
		row := newLotSnapshot(lot, position.Quote.Bid, createdAt, position.RecentlyBought)
		row.PositionIdx = positionIdx
		position.LotIdxs = append(position.LotIdxs, len(lots))
		lots = append(lots, row)
	}

	// Brokers list lots in no set order; sort by lot ID so plans repeat exactly.
	for i := range positions {
		slices.SortFunc(positions[i].LotIdxs, func(left, right int) int {
			return cmp.Compare(lots[left].Id, lots[right].Id)
		})
	}
	return lots
}

// Tax on selling one share of each lot, by lot row. All zero without rates, as for an account that is not taxable.
func TaxPerShare(lots []LotSnapshot, rates *config.TaxSettings) []decimal.Decimal {
	taxPerShare := make([]decimal.Decimal, len(lots))
	if rates == nil {
		return taxPerShare
	}
	for i, lot := range lots {
		if lot.IsWashSale {
			continue
		}
		rate := rates.ShortTermGainRate
		if lot.LongTerm {
			rate = rates.LongTermGainRate
		}
		taxPerShare[i] = lot.GainPerShare.Mul(rate)
	}
	return taxPerShare
}

// Compare two lots by tax per dollar raised selling at their bids,
// exactly across share prices by scaling each tax per share with the other lot's bid.
func CompareTaxPerDollar(snapshot Snapshot, taxPerShare []decimal.Decimal, leftLotIdx, rightLotIdx int) int {
	leftBid := snapshot.Positions[snapshot.Lots[leftLotIdx].PositionIdx].Quote.Bid
	rightBid := snapshot.Positions[snapshot.Lots[rightLotIdx].PositionIdx].Quote.Bid
	return taxPerShare[leftLotIdx].Mul(rightBid).Cmp(taxPerShare[rightLotIdx].Mul(leftBid))
}

func isLongTerm(timeAcquired, saleDate time.Time) bool {
	acquiredYear, acquiredMonth, acquiredDay := timeAcquired.In(saleDate.Location()).Date()
	soldYear, soldMonth, soldDay := saleDate.Date()

	// The holding period starts the day after acquisition; the sale day counts.
	// UTC date-only values keep time of day and DST out of the calculation.
	holdingStart := time.Date(acquiredYear, acquiredMonth, acquiredDay+1, 0, 0, 0, 0, time.UTC)

	// Shift the inclusive sale date back one year instead of advancing the start.
	// This preserves the boundary when either year includes February 29.
	saleOneYearEarlier := time.Date(soldYear-1, soldMonth, soldDay, 0, 0, 0, 0, time.UTC)
	return !saleOneYearEarlier.Before(holdingStart)
}
