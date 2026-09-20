package config

import "github.com/shopspring/decimal"

const (
	DecimalDivisionPrecision int32 = 19

	// How many days after a sale before you may buy without incurring a wash-sale?
	WashSaleWindowDays = 31

	CanBuyFractionalShares = false

	NoTicker = ""
)

var (
	DecimalOne = decimal.NewFromInt(1)

	// Delay (ms) before each API attempt; length sets the attempt limit, with
	// no schedule enforcing a single attempt without retries
	MsPerRetryAPI = []int{0, 250, 500}

	// If not, how small a fractional share you would round to a whole share?
	MinFractionalShareToRound = decimal.RequireFromString("0.7")

	// How much asset allocation deviation, percentage-wise, do you tolerate?
	MaxPctAllocDeviationAllowed = decimal.RequireFromString("0.02") // or 2%

	// Harvest lots with P/L at or below this negative threshold.
	MinPctLossToHarvest = decimal.RequireFromString("-0.005") // -0.5%
)
