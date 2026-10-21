package config

import (
	"github.com/shopspring/decimal"
)

type User struct {
	Tax TaxSettings `yaml:"tax"`
}

type TaxSettings struct {
	ShortTermGainRate *decimal.Decimal `yaml:"short_term_gain_rate" validate:"required,gte=0,lte=1"`
	LongTermGainRate  *decimal.Decimal `yaml:"long_term_gain_rate" validate:"required,gte=0,lte=1"`
}

func (settings TaxSettings) Validate() error {
	return validateSettings(settings, "tax")
}
