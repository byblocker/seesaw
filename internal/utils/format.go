package utils

import (
	"strings"

	"github.com/shopspring/decimal"
)

func FormatMoney(amount decimal.Decimal) string {
	return formatDollars(amount, amount.Abs().StringFixed(2))
}

func FormatPercent(ratio decimal.Decimal) string {
	return ratio.Shift(2).StringFixed(2) + "%"
}

func FormatPrice(amount decimal.Decimal) string {
	if amount.Equal(amount.Round(2)) {
		return FormatMoney(amount)
	}
	return formatDollars(amount, amount.Abs().String())
}

func formatDollars(amount decimal.Decimal, digits string) string {
	whole, fraction, _ := strings.Cut(digits, ".")
	prefix := "$"
	if amount.IsNegative() {
		prefix = "-$"
	}
	var output strings.Builder
	output.WriteString(prefix)
	for index, digit := range whole {
		if index > 0 && (len(whole)-index)%3 == 0 {
			output.WriteByte(',')
		}
		output.WriteRune(digit)
	}
	output.WriteByte('.')
	output.WriteString(fraction)
	return output.String()
}
