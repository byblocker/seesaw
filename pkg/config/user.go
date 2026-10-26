package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type User struct {
	// Nil when no tax rates are configured; required for taxable portfolios.
	Tax         *TaxSettings
	Rebalance   RebalanceSettings
	Liquidation LiquidationSettings
}

type userYAML struct {
	Tax         *taxYAML        `yaml:"tax"`
	Rebalance   rebalanceYAML   `yaml:"rebalance"`
	Liquidation liquidationYAML `yaml:"liquidation"`
}

func NewUser(tax *TaxSettings, rebalance RebalanceSettings, liquidation LiquidationSettings) (*User, error) {
	if tax != nil {
		if err := tax.Validate(); err != nil {
			return nil, err
		}
	}
	if err := rebalance.Validate(); err != nil {
		return nil, err
	}
	if err := liquidation.Validate(); err != nil {
		return nil, err
	}
	return &User{Tax: tax, Rebalance: rebalance, Liquidation: liquidation}, nil
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

	config, err := ParseUser(file)
	if err != nil {
		return nil, fmt.Errorf("loading user config %s: %w", path, err)
	}
	return config, nil
}

func ParseUser(file io.Reader) (*User, error) {
	var decoded userYAML
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	var extraDocument any
	if err := decoder.Decode(&extraDocument); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("failed to parse user config: %w", err)
		}
		return nil, fmt.Errorf("user config must contain exactly one YAML document")
	}

	var tax *TaxSettings
	if decoded.Tax != nil {
		built, err := decoded.Tax.build()
		if err != nil {
			return nil, err
		}
		tax = &built
	}
	rebalance, err := decoded.Rebalance.build()
	if err != nil {
		return nil, err
	}
	liquidation, err := decoded.Liquidation.build()
	if err != nil {
		return nil, err
	}
	return NewUser(tax, rebalance, liquidation)
}
