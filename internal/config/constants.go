package config

import "github.com/shopspring/decimal"

const (
	// Decimal places retained during division. "19" limits intermediate precision loss.
	DivisionDecimalPlaces int32 = 19

	// Days to lookback for transaction history. "31" covers the IRS specifications for
	// avoiding wash-sales during the `rebalance` workflow
	WashSaleWindowDays = 31

	// Allow fractional buys? ETRADE "supports" fractional shares of certain securities,
	// so we default to "false"
	CanBuyFractionalShares = false

	// Ticker for no eligible purchase. DO NOT EDIT.
	NoTicker = ""
)

var (
	// Shared decimal used for increments and 100% decimals. DO NOT EDIT.
	DecimalOne = decimal.NewFromInt(1)

	// Milliseconds before each HTTP attempt. {0, 250, 500} allows two short retries
	// after an immediate attempt to recover from transient GET connection failures.
	MsPerRetryAPI = []int{0, 250, 500}

	// Minimum fractional remainder for rounding a buy up. 0.7 favors rounding down
	// to limit overshooting the target allocation.
	MinFractionalShareToRound = decimal.RequireFromString("0.7")

	// Max tolerance of deviation from asset's `target_allocation`; provides extra
	// purchase allowance during `rebalance` workflows. 0.02, or 2%, gives whole-share
	// rounding up to 2% of portfolio value in headroom, subject to available cash.
	MaxPctAllocDeviationAllowed = decimal.RequireFromString("0.02")

	// Minimum fractional loss required to initiate tax-loss-harvesting. -0.005, or -0.5%,
	// excludes harvesting small fluctuations.
	MinPctLossToHarvest = decimal.RequireFromString("-0.005")
)
