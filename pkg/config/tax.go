package config

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type User struct {
	Tax TaxSettings `yaml:"tax"`
}

type TaxSettings struct {
	ShortTermGainRate *decimal.Decimal `yaml:"short_term_gain_rate"`
	LongTermGainRate  *decimal.Decimal `yaml:"long_term_gain_rate"`
}

func (settings TaxSettings) Validate() error {
	one := decimal.NewFromInt(1)
	for _, field := range []struct {
		name string
		rate *decimal.Decimal
	}{
		{"tax.short_term_gain_rate", settings.ShortTermGainRate},
		{"tax.long_term_gain_rate", settings.LongTermGainRate},
	} {
		if field.rate == nil {
			return fmt.Errorf("missing required %s", field.name)
		}
		if field.rate.IsNegative() || field.rate.GreaterThan(one) {
			return fmt.Errorf("%s must be between 0 and 1", field.name)
		}
	}
	return nil
}
