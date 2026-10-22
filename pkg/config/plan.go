package config

import (
	"bytes"
	"fmt"
	"io"
	"iter"
	"os"
	"seesaw/internal/collections"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

type Plan struct {
	Broker    string      `yaml:"broker"`
	AccountId string      `yaml:"accountId"`
	IsTaxable *bool       `yaml:"is_taxable"`
	Assets    []AssetPlan `yaml:"assets"`
}

func (plan *Plan) Tickers() iter.Seq[string] {
	return collections.FlatMap(slices.Values(plan.Assets), AssetPlan.Tickers)
}

type AssetPlan struct {
	PrimaryTicker    string          `yaml:"primary_ticker" validate:"required,uppercase"`
	AltTicker        string          `yaml:"alternate_ticker" validate:"omitempty,uppercase"`
	TargetAllocation decimal.Decimal `yaml:"target_allocation" validate:"gt=0,lte=1"`
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

func LoadPlan(path string) (*Plan, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening portfolio plan %s: %w", path, err)
	}
	defer file.Close()

	plan, err := ParsePlan(file)
	if err != nil {
		return nil, fmt.Errorf("loading portfolio plan %s: %w", path, err)
	}
	return plan, nil
}

func ParsePlan(file io.Reader) (*Plan, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read portfolio plan at %w", err,
		)
	}

	plan := &Plan{}
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

	if plan.IsTaxable == nil {
		return nil, fmt.Errorf("missing required is_taxable")
	}

	if err := normalizeAssets(plan.Assets); err != nil {
		return nil, err
	}

	return plan, nil
}

func normalizeAssets(assets []AssetPlan) error {
	totalAllocation := decimal.Zero
	seenTickers := collections.NewSet[string]()

	for i := range assets {
		assets[i].PrimaryTicker = strings.TrimSpace(assets[i].PrimaryTicker)
		assets[i].AltTicker = strings.TrimSpace(assets[i].AltTicker)
	}
	if err := validateAssets(assets); err != nil {
		return err
	}

	for _, asset := range assets {
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
