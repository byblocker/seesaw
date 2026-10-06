package utils

import "github.com/shopspring/decimal"

const DivisionDecimalPlaces int32 = 19

func ClampNonNegative(value decimal.Decimal) decimal.Decimal {
	return decimal.Max(decimal.Zero, value)
}
