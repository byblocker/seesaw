package config

import (
	"github.com/shopspring/decimal"
)

const (
	DefaultMinFractionalShareToRound   = "0.7"
	DefaultMaxPctAllocDeviationAllowed = "0.02"
	DefaultMinPctLossToHarvest         = "-0.005"

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
