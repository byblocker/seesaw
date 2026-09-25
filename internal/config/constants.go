package config

import (
	"github.com/shopspring/decimal"
)

const (
	// Decimal places retained during division. "19" limits intermediate precision loss.
	DivisionDecimalPlaces int32 = 19

	// Days to lookback for transaction history. "31" covers the IRS specifications for
	// avoiding wash-sales during the `rebalance` workflow
	WashSaleWindowDays = 31

	// Ticker for no eligible purchase. DO NOT EDIT.
	NoTicker = ""
)

// Shared decimal used for increments and 100% decimals. DO NOT EDIT.
var DecimalOne = decimal.NewFromInt(1)

var (
	// Minimum remainder to round a whole-share purchase up.
	MinFractionalShareToRound = decimal.RequireFromString("0.7")

	// Extra whole-share purchase allowance as a fraction of portfolio value,
	// still limited by available cash.
	MaxPctAllocDeviationAllowed = decimal.RequireFromString("0.02")

	// Harvest lots at or below this return; -0.005 means a loss of at least 0.5%.
	MinPctLossToHarvest = decimal.RequireFromString("-0.005")
)
