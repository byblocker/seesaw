package format

import (
	"strings"

	"github.com/shopspring/decimal"
)

func Money(amount decimal.Decimal) string {
	return dollars(amount, amount.Abs().StringFixed(2))
}

func Percent(ratio decimal.Decimal) string {
	return ratio.Shift(2).StringFixed(2) + "%"
}

func Price(amount decimal.Decimal) string {
	if amount.Equal(amount.Round(2)) {
		return Money(amount)
	}
	return dollars(amount, amount.Abs().String())
}

func dollars(amount decimal.Decimal, digits string) string {
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

// The last four characters of an account ID, or the whole ID when shorter.
func AccountSuffix(id string) string {
	if len(id) > 4 {
		return id[len(id)-4:]
	}
	return id
}
