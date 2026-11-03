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
		LongTerm:     lot.IsLongTermAt(createdAt),
		IsWashSale:   recentlyBought && gainPerShare.IsNegative(),
	}
}
