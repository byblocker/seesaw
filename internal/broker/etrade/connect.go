package etrade

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"seesaw/internal/keyring"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"strings"
)

const Name = "ETRADE"

const etradeConfigKey = "etrade_config"

const productionAPIBaseURL = "https://api.etrade.com/v1"

type Config struct {
	APIBaseURL string
	APIKey     string
	APISecret  string
}

func (config Config) Validate() error {
	if strings.TrimSpace(config.APIKey) == "" {
		return fmt.Errorf("ETrade APIKey must not be empty")
	}

	if strings.TrimSpace(config.APISecret) == "" {
		return fmt.Errorf("ETrade APISecret must not be empty")
	}

	baseURL, err := url.Parse(config.APIBaseURL)
	if err != nil || baseURL.Hostname() == "" || (baseURL.Scheme != "https" && baseURL.Scheme != "http") {
		return fmt.Errorf("ETrade APIBaseURL must be an absolute HTTP or HTTPS URL")
	}
	if baseURL.User != nil || baseURL.RawQuery != "" || baseURL.ForceQuery || baseURL.Fragment != "" {
		return fmt.Errorf("ETrade APIBaseURL must not contain user information, a query, or a fragment")
	}

	return nil
}

type Broker struct{}

// Prompt for and save the configuration. Saved is false when the user keeps the existing one.
func (Broker) Setup() (saved bool, err error) {
	exists, err := keyring.Exists(etradeConfigKey)
	if err != nil {
		return false, fmt.Errorf("could not check saved ETrade configuration; check OS keyring access: %w", err)
	}
	if exists {
		answer, err := utils.AskForInput("ETrade configuration already exists. Overwrite it? [y/N]: ")
		if err != nil {
			return false, err
		}
		if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
			return false, nil
		}
	}

	config, err := promptForConfig()
	if err != nil {
		return false, err
	}

	if err := config.Validate(); err != nil {
		return false, err
	}

	if err := keyring.Set(etradeConfigKey, config); err != nil {
		return false, fmt.Errorf("could not save ETrade configuration; check OS keyring access: %w", err)
	}
	return true, nil
}

func (Broker) Connect(ctx context.Context) (models.BrokerClient, error) {
	config, err := keyring.Get[Config](etradeConfigKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, models.ErrBrokerNotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("%w: could not load ETrade configuration; check OS keyring access: %w", models.ErrBrokerNotConfigured, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("%w: invalid saved ETrade configuration: %w", models.ErrBrokerNotConfigured, err)
	}

	httpClient, err := authenticate(ctx, config.APIKey, config.APISecret, config.APIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", Name, err)
	}
	return &Client{httpClient: httpClient, apiBaseURL: config.APIBaseURL}, nil
}

func promptForConfig() (Config, error) {
	baseURL, err := utils.AskForInput(fmt.Sprintf("ETrade API base URL [%s]: ", productionAPIBaseURL))
	if err != nil {
		return Config{}, err
	}

	if baseURL == "" {
		baseURL = productionAPIBaseURL
	}

	apiKey, err := utils.AskForSecret("ETrade API key: ")
	if err != nil {
		return Config{}, err
	}

	apiSecret, err := utils.AskForSecret("ETrade API secret: ")
	if err != nil {
		return Config{}, err
	}

	return Config{
		APIBaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		APISecret:  apiSecret,
	}, nil
}
