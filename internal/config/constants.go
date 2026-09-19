package config

import "github.com/quagmt/udecimal"

const (
	// How many days after a sale before you may buy without incurring a wash-sale?
	WashSaleWindowDays = 31

	CanBuyFractionalShares = false
)

var (
	// If not, how small a fractional share you would round to a whole share?
	MinFractionalShareToRound = udecimal.MustParse("0.7")

	// How much asset allocation deviation, percentage-wise, do you tolerate?
	MaxAllocPctDeviationAllowed = udecimal.MustParse("0.02")
)
