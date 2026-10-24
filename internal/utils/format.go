package utils

import (
	"strings"

	"github.com/shopspring/decimal"
)

func FormatMoney(amount decimal.Decimal) string {
	prefix := "$"
	if amount.IsNegative() {
		prefix = "-$"
	}
	formatted := amount.Abs().StringFixed(2)
	decimalPoint := len(formatted) - 3
	var output strings.Builder
	output.WriteString(prefix)
	for index, digit := range formatted[:decimalPoint] {
		if index > 0 && (decimalPoint-index)%3 == 0 {
			output.WriteByte(',')
		}
		output.WriteRune(digit)
	}
	output.WriteString(formatted[decimalPoint:])
	return output.String()
}

func FormatPercent(ratio decimal.Decimal) string {
	return ratio.Shift(2).StringFixed(2) + "%"
}

func FormatPrice(amount decimal.Decimal) string {
	if amount.Equal(amount.Round(2)) {
		return FormatMoney(amount)
	}

	if amount.IsNegative() {
		return "-$" + amount.Abs().String()
	}
	return "$" + amount.String()
}
