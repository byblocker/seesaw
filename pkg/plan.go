package pkg

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"seesaw/internal/collections"
	"seesaw/pkg/models"
	"strings"

	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

func LoadPortfolioPlan(path string) (*models.PortfolioPlan, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening portfolio plan %s: %w", path, err)
	}
	defer file.Close()

	plan, err := ParsePortfolioPlan(file)
	if err != nil {
		return nil, fmt.Errorf("loading portfolio plan %s: %w", path, err)
	}
	return plan, nil
}

func ParsePortfolioPlan(file io.Reader) (*models.PortfolioPlan, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read portfolio plan at %w", err,
		)
	}

	plan := &models.PortfolioPlan{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(plan); err != nil {
		return nil, fmt.Errorf(
			"failed to parse portfolio plan at %w", err,
		)
	}
	var extraDocument any
	if err := decoder.Decode(&extraDocument); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("failed to parse portfolio plan: %w", err)
		}
		return nil, fmt.Errorf("portfolio plan must contain exactly one YAML document")
	}

	if err := plan.Rebalance.Validate(); err != nil {
		return nil, err
	}

	if plan.IsTaxable == nil {
		return nil, fmt.Errorf("missing required is_taxable")
	}

	if err := plan.Liquidation.Validate(); err != nil {
		return nil, err
	}

	if err := normalizeAssets(plan.Assets); err != nil {
		return nil, err
	}

	return plan, nil
}

func normalizeAssets(assets []models.AssetPlan) error {
	totalAllocation := decimal.Zero
	seenTickers := collections.NewSet[string]()

	for i := range assets {
		asset := &assets[i]

		asset.PrimaryTicker = strings.TrimSpace(asset.PrimaryTicker)
		asset.AltTicker = strings.TrimSpace(asset.AltTicker)

		if err := asset.Validate(); err != nil {
			return err
		}

		for ticker := range asset.Tickers() {
			if seenTickers.Contains(ticker) {
				return fmt.Errorf("duplicate ticker %q", ticker)
			}

			seenTickers.Add(ticker)
		}

		totalAllocation = totalAllocation.Add(asset.TargetAllocation)
	}

	if !totalAllocation.Equal(decimal.NewFromInt(1)) {
		return fmt.Errorf("target allocations must sum to 1.0; got %s", totalAllocation.String())
	}

	return nil
}
