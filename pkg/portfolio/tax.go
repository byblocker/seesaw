package portfolio

import (
	"seesaw/internal/calendar"
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
