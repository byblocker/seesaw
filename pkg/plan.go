package pkg

import (
	"fmt"
	"io"
	"iter"
	"seesaw/internal/collections"
	"seesaw/internal/config"
	"seesaw/internal/utils"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

type PortfolioPlan struct {
	AccountId string      `yaml:"accountId"`
	Assets    []AssetPlan `yaml:"assets"`
}

func (plan *PortfolioPlan) Tickers() iter.Seq[string] {
	return collections.FlatMap(slices.Values(plan.Assets), AssetPlan.Tickers)
}

type AssetPlan struct {
	PrimaryTicker    string          `yaml:"primary_ticker"`
	AltTicker        string          `yaml:"alternate_ticker"`
	TargetAllocation decimal.Decimal `yaml:"target_allocation"`
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

	totalAllocation := decimal.Zero
	seenTickers := collections.NewSet[string]()

	for i := range plan.Assets {
		asset := &plan.Assets[i]
		asset.PrimaryTicker = strings.TrimSpace(asset.PrimaryTicker)
		asset.AltTicker = strings.TrimSpace(asset.AltTicker)

		if err := validateAsset(asset); err != nil {
			return nil, err
		}

		for ticker := range asset.Tickers() {
			if seenTickers.Contains(ticker) {
				return nil, fmt.Errorf("duplicate ticker %q", ticker)
			}

			seenTickers.Add(ticker)
		}

		totalAllocation = totalAllocation.Add(asset.TargetAllocation)
	}

	if !totalAllocation.Equal(config.DecimalOne) {
		return nil, fmt.Errorf("target allocations must sum to 1.0; got %s", totalAllocation.String())
	}

	return plan, nil
}

func validateAsset(asset *AssetPlan) error {
	if asset.PrimaryTicker == "" {
		return fmt.Errorf("missing required primary_ticker")
	}

	for ticker := range asset.Tickers() {
		if ticker != strings.ToUpper(ticker) {
			return fmt.Errorf("ticker %q must be uppercase", ticker)
		}
	}

	allocTooLarge := asset.TargetAllocation.Cmp(config.DecimalOne) > 0
	allocNegative := asset.TargetAllocation.Cmp(decimal.Zero) <= 0

	if allocTooLarge || allocNegative {
		return fmt.Errorf(
			"invalid target allocation: %s",
			asset.TargetAllocation.StringFixed(2),
		)
	}

	return nil
}
