package config

import (
	"fmt"
	"io"
	"os"
	"seesaw/internal/utils"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

type Plan struct {
	Broker    string
	AccountId string
	IsTaxable bool
	Settings  Settings
	Assets    []AssetPlan
}

type planYAML struct {
	Broker    string       `yaml:"broker"`
	AccountId string       `yaml:"accountId"`
	IsTaxable *bool        `yaml:"is_taxable"`
	Settings  settingsYAML `yaml:"settings"`
	Assets    []AssetPlan  `yaml:"assets"`
}

func (plan *Plan) AccountLabel() string {
	return fmt.Sprintf("%s #%s", strings.ToUpper(plan.Broker), utils.AccountSuffix(plan.AccountId))
}

func (plan *Plan) Tickers() []string {
	var tickers []string
	for _, asset := range plan.Assets {
		tickers = append(tickers, asset.Tickers()...)
	}
	return tickers
}

type AssetPlan struct {
	PrimaryTicker    string          `yaml:"primary_ticker" validate:"required,uppercase"`
	AltTicker        string          `yaml:"alternate_ticker" validate:"omitempty,uppercase"`
	TargetAllocation decimal.Decimal `yaml:"target_allocation" validate:"gt=0,lte=1"`
}

func (asset AssetPlan) Tickers() []string {
	if asset.AltTicker == "" {
		return []string{asset.PrimaryTicker}
	}
	return []string{asset.PrimaryTicker, asset.AltTicker}
}

func NewPlan(broker, accountId string, isTaxable bool, settings Settings, assets []AssetPlan) (*Plan, error) {
	broker, accountId = strings.TrimSpace(broker), strings.TrimSpace(accountId)
	if broker == "" {
		return nil, fmt.Errorf("missing required broker")
	}
	if accountId == "" {
		return nil, fmt.Errorf("missing required accountId")
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	assets = slices.Clone(assets)
	for i := range assets {
		asset := &assets[i]
		asset.PrimaryTicker = strings.TrimSpace(asset.PrimaryTicker)
		asset.AltTicker = strings.TrimSpace(asset.AltTicker)
		if err := utils.ValidateFields(*asset, fmt.Sprintf("assets[%d].", i)); err != nil {
			return nil, err
		}
	}
	if err := validateAllocations(assets); err != nil {
		return nil, err
	}

	return &Plan{
		Broker:    broker,
		AccountId: accountId,
		IsTaxable: isTaxable,
		Settings:  settings,
		Assets:    assets,
	}, nil
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

func ParsePlan(reader io.Reader) (*Plan, error) {
	var decoded planYAML
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("failed to parse portfolio plan: %w", err)
	}
	if err := requireSingleDocument(decoder, "portfolio plan"); err != nil {
		return nil, err
	}

	if decoded.IsTaxable == nil {
		return nil, fmt.Errorf("missing required is_taxable")
	}
	settings, err := decoded.Settings.build()
	if err != nil {
		return nil, err
	}

	return NewPlan(decoded.Broker, decoded.AccountId, *decoded.IsTaxable, settings, decoded.Assets)
}

// Reject any YAML document after the one the decoder already read.
func requireSingleDocument(decoder *yaml.Decoder, description string) error {
	var extraDocument any
	err := decoder.Decode(&extraDocument)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", description, err)
	}
	return fmt.Errorf("%s must contain exactly one YAML document", description)
}

// Tickers must be unique across assets, and target allocations must sum to 1.
func validateAllocations(assets []AssetPlan) error {
	totalAllocation := decimal.Zero
	seenTickers := map[string]bool{}
	for _, asset := range assets {
		for _, ticker := range asset.Tickers() {
			if seenTickers[ticker] {
				return fmt.Errorf("duplicate ticker %q", ticker)
			}

			seenTickers[ticker] = true
		}

		totalAllocation = totalAllocation.Add(asset.TargetAllocation)
	}

	if !totalAllocation.Equal(decimal.NewFromInt(1)) {
		return fmt.Errorf("target allocations must sum to 1.0; got %s", totalAllocation.String())
	}

	return nil
}
