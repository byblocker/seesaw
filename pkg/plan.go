package pkg

import (
	"fmt"
	"io"
	"seesaw/internal/collections"
	"seesaw/pkg/models"
	"strings"

	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

func ParsePortfolioPlan(file io.Reader) (*models.PortfolioPlan, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read portfolio plan at %w", err,
		)
	}

	plan := &models.PortfolioPlan{}
	if err := yaml.Unmarshal(data, plan); err != nil {
		return nil, fmt.Errorf(
			"failed to parse portfolio plan at %w", err,
		)
	}

	if err := plan.Settings.Validate(); err != nil {
		return nil, err
	}

	totalAllocation := decimal.Zero
	seenTickers := collections.NewSet[string]()

	for i := range plan.Assets {
		asset := &plan.Assets[i]
		asset.PrimaryTicker = strings.TrimSpace(asset.PrimaryTicker)
		asset.AltTicker = strings.TrimSpace(asset.AltTicker)

		if err := asset.Validate(); err != nil {
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

	if !totalAllocation.Equal(decimal.NewFromInt(1)) {
		return nil, fmt.Errorf("target allocations must sum to 1.0; got %s", totalAllocation.String())
	}

	return plan, nil
}
