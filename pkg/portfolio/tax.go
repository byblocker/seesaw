package portfolio

import (
	"seesaw/pkg/config"
	"time"

	"github.com/shopspring/decimal"
)

const (
	WashSaleWindowDays     = 30
	transactionHistoryDays = WashSaleWindowDays + 1
)

func isLongTerm(timeAcquired, saleDate time.Time) bool {
	// The holding period starts the day after acquisition; the sale day counts.
	holdingStart := utcDate(timeAcquired.In(saleDate.Location())).AddDate(0, 0, 1)

	// Shift the inclusive sale date back one year instead of advancing the start.
	// This preserves the boundary when either year includes February 29.
	saleOneYearEarlier := utcDate(saleDate).AddDate(-1, 0, 0)
	return !saleOneYearEarlier.Before(holdingStart)
}

func daysBetween(from, to time.Time) int {
	fromDay := utcDate(from.In(to.Location()))
	toDay := utcDate(to)
	return int(toDay.Sub(fromDay).Hours() / 24)
}

// The moment's calendar date as midnight UTC, so date math ignores time of day and DST.
func utcDate(moment time.Time) time.Time {
	year, month, day := moment.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
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
