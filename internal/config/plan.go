package config

import (
	"fmt"
	"io"
	"seesaw/internal/utils"

	"github.com/quagmt/udecimal"
)

type PortfolioPlan struct {
	AccountId string  `yaml:"accountId"`
	Assets    []Asset `yaml:"assets"`
}

type Asset struct {
	PrimaryTicker    string           `yaml:"primary_ticker"`
	AltTicker        string           `yaml:"alternate_ticker"`
	TargetAllocation udecimal.Decimal `yaml:"target_allocation"`
}

func ParsePortfolioPlan(file io.Reader) (*PortfolioPlan, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read portfolio plan at %w", err,
		)
	}

	plan, err := utils.YAMLToStruct[PortfolioPlan](data)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse portfolio plan at %w", err,
		)
	}

	for _, asset := range plan.Assets {
		if err := validateAsset(&asset); err != nil {
			return nil, err
		}
	}

	return plan, nil
}

func validateAsset(asset *Asset) error {
	if asset.PrimaryTicker == "" {
		return fmt.Errorf("missing required primary_ticker")
	}

	allocTooLarge := asset.TargetAllocation.GreaterThan(udecimal.One)
	allocNegative := asset.TargetAllocation.LessThanOrEqual(udecimal.Zero)

	if allocTooLarge || allocNegative {
		return fmt.Errorf(
			"invalid target allocation: %s",
			asset.TargetAllocation.StringFixed(2),
		)
	}

	return nil
}
