package config

import (
	"fmt"
	"io"
	"os"
	"seesaw/internal/validate"

	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

type User struct {
	// Nil when no tax rates are configured; required for taxable portfolios.
	Tax *TaxSettings
}

type userYAML struct {
	Tax *taxYAML `yaml:"tax"`
}

type TaxSettings struct {
	ShortTermGainRate decimal.Decimal
	LongTermGainRate  decimal.Decimal
}

// Tax rates as written in YAML, before they are checked. Pointers tell a missing key from an explicit zero.
type taxYAML struct {
	ShortTermGainRate *decimal.Decimal `yaml:"short_term_gain_rate" validate:"required,gte=0,lte=1"`
	LongTermGainRate  *decimal.Decimal `yaml:"long_term_gain_rate" validate:"required,gte=0,lte=1"`
}

func (raw taxYAML) build() (TaxSettings, error) {
	if err := validate.Fields(raw, "tax."); err != nil {
		return TaxSettings{}, err
	}
	return TaxSettings{
		ShortTermGainRate: *raw.ShortTermGainRate,
		LongTermGainRate:  *raw.LongTermGainRate,
	}, nil
}

func (user *User) TaxRates(isTaxable bool) (*TaxSettings, error) {
	if !isTaxable {
		return nil, nil
	}
	if user.Tax == nil {
		return nil, fmt.Errorf("portfolio is taxable, but user config has no tax rates")
	}
	return user.Tax, nil
}

func LoadUser(path string) (*User, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening user config %s: %w", path, err)
	}
	defer file.Close()

	user, err := ParseUser(file)
	if err != nil {
		return nil, fmt.Errorf("loading user config %s: %w", path, err)
	}
	return user, nil
}

func ParseUser(reader io.Reader) (*User, error) {
	var decoded userYAML
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	// An empty file is a config with no tax rates.
	if err := decoder.Decode(&decoded); err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	if err := requireSingleDocument(decoder, "user config"); err != nil {
		return nil, err
	}

	var tax *TaxSettings
	if decoded.Tax != nil {
		built, err := decoded.Tax.build()
		if err != nil {
			return nil, err
		}
		tax = &built
	}
	return &User{Tax: tax}, nil
}
