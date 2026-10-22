package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type User struct {
	Tax         TaxSettings         `yaml:"tax"`
	Rebalance   RebalanceSettings   `yaml:"rebalance"`
	Liquidation LiquidationSettings `yaml:"liquidation"`
}

func NewUser(tax TaxSettings, rebalance RebalanceSettings, liquidation LiquidationSettings) (*User, error) {
	if err := tax.Validate(); err != nil {
		return nil, err
	}
	if err := rebalance.Validate(); err != nil {
		return nil, err
	}
	if err := liquidation.Validate(); err != nil {
		return nil, err
	}
	return &User{Tax: tax, Rebalance: rebalance, Liquidation: liquidation}, nil
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
	config := &User{}
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	var extraDocument any
	if err := decoder.Decode(&extraDocument); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("failed to parse user config: %w", err)
		}
		return nil, fmt.Errorf("user config must contain exactly one YAML document")
	}

	return NewUser(config.Tax, config.Rebalance, config.Liquidation)
}
