package portfolio

import (
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"time"

	"github.com/shopspring/decimal"
)

type LotSnapshot struct {
	models.PositionLot

	// Selling at the snapshot's bid on its date
	GainPerShare decimal.Decimal
	GainPct      decimal.Decimal // Fraction of the price paid
	LongTerm     bool
	IsWashSale   bool
}

func newLotSnapshot(lot models.PositionLot, bid decimal.Decimal, createdAt time.Time, recentlyBought bool) LotSnapshot {
	gainPerShare := bid.Sub(lot.PricePaid)
	gainPct := decimal.Zero
	if lot.PricePaid.IsPositive() {
		gainPct, _ = gainPerShare.QuoRem(lot.PricePaid, utils.DivisionDecimalPlaces)
	}

	return LotSnapshot{
		PositionLot:  lot,
		GainPerShare: gainPerShare,
		GainPct:      gainPct,
		LongTerm:     isLongTerm(lot.TimeAcquired, createdAt),
		IsWashSale:   recentlyBought && gainPerShare.IsNegative(),
	}
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
