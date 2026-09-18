package pkg

import (
	"fmt"
	"io"
	"iter"
	"seesaw/internal"
	"seesaw/internal/utils"
	"slices"
	"strings"

	"github.com/quagmt/udecimal"
)

type PortfolioPlan struct {
	AccountId string      `yaml:"accountId"`
	Assets    []AssetPlan `yaml:"assets"`
}

func (plan *PortfolioPlan) Tickers() iter.Seq[string] {
	return internal.FlatMap(slices.Values(plan.Assets), AssetPlan.Tickers)
}

type AssetPlan struct {
	PrimaryTicker    string           `yaml:"primary_ticker"`
	AltTicker        string           `yaml:"alternate_ticker"`
	TargetAllocation udecimal.Decimal `yaml:"target_allocation"`
}

func (asset AssetPlan) Tickers() iter.Seq[string] {
	return func(yield func(string) bool) {
		yieldTicker := func(ticker string) bool {
			return ticker == "" || yield(ticker)
		}
		if !yieldTicker(asset.PrimaryTicker) || !yieldTicker(asset.AltTicker) {
			return
		}
	}
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

	for i := range plan.Assets {
		asset := &plan.Assets[i]
		asset.PrimaryTicker = strings.TrimSpace(asset.PrimaryTicker)
		asset.AltTicker = strings.TrimSpace(asset.AltTicker)
		if err := validateAsset(asset); err != nil {
			return nil, err
		}
	}

	return plan, nil
}

func validateAsset(asset *AssetPlan) error {
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
