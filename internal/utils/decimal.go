package utils

import "github.com/shopspring/decimal"

func ClampNonNegative(value decimal.Decimal) decimal.Decimal {
	return decimal.Max(decimal.Zero, value)
}
