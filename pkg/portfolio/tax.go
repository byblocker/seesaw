package portfolio

import (
	"seesaw/internal/calendar"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"time"

	"github.com/shopspring/decimal"
)

const (
	washSaleWindowDays     = 30
	transactionHistoryDays = washSaleWindowDays + 1
)

func isLongTerm(timeAcquired, saleDate time.Time) bool {
	// The holding period starts the day after acquisition; the sale day counts.
	holdingStart := calendar.UTCDate(timeAcquired.In(saleDate.Location())).AddDate(0, 0, 1)

	// Shift the inclusive sale date back one year instead of advancing the start.
	// This preserves the boundary when either year includes February 29.
	saleOneYearEarlier := calendar.UTCDate(saleDate).AddDate(-1, 0, 0)
	return !saleOneYearEarlier.Before(holdingStart)
}

// Tax on selling one share of the lot. Zero without rates, as for an account that is not taxable.
func (lot LotSnapshot) taxPerShare(rates *config.TaxSettings) decimal.Decimal {
	if rates == nil || lot.IsWashSale {
		return decimal.Zero
	}
	rate := rates.ShortTermGainRate
	if lot.LongTerm {
		rate = rates.LongTermGainRate
	}
	return lot.GainPerShare.Mul(rate)
}

// Tax on selling one share of each lot, by lot row. All zero without rates, as for an account that is not taxable.
func TaxPerShare(lots []Lot, rates *config.TaxSettings) []decimal.Decimal {
	taxPerShare := make([]decimal.Decimal, len(lots))
	for lotIdx, lot := range lots {
		taxPerShare[lotIdx] = lot.Snapshot.taxPerShare(rates)
	}
	return taxPerShare
}

// Tax per dollar raised selling each lot at its bid, by lot row. Truncating the quotient
// never reorders lots, so sorting by it matches the exact ratio up to ties below its precision.
func TaxPerDollar(lots []Lot, rates *config.TaxSettings) []decimal.Decimal {
	taxPerDollar := TaxPerShare(lots, rates)
	for lotIdx, lot := range lots {
		taxPerDollar[lotIdx], _ = taxPerDollar[lotIdx].QuoRem(lot.Snapshot.Bid, utils.DivisionDecimalPlaces)
	}
	return taxPerDollar
}
