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

// Tax on selling one share of each lot, by lot row. All zero without rates, as for an account that is not taxable.
func TaxPerShare(lots []Lot, rates *config.TaxSettings) []decimal.Decimal {
	taxPerShare := make([]decimal.Decimal, len(lots))
	if rates == nil {
		return taxPerShare
	}
	for lotIdx, lot := range lots {
		if lot.Snapshot.IsWashSale {
			continue
		}
		rate := rates.ShortTermGainRate
		if lot.Snapshot.LongTerm {
			rate = rates.LongTermGainRate
		}
		taxPerShare[lotIdx] = lot.Snapshot.GainPerShare.Mul(rate)
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
