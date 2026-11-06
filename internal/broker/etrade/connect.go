package etrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"seesaw/internal/broker"
	"seesaw/pkg/models"
	"seesaw/pkg/secrets"
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

type Broker struct {
	Secrets secrets.Store // Holds the configuration and access token
}

// Prompt for and save the configuration. Saved is false when the user keeps the existing one.
func (etradeBroker Broker) Setup(ctx context.Context) (saved bool, err error) {
	exists, err := secrets.Exists(ctx, etradeBroker.Secrets, etradeConfigKey)
	if err != nil {
		return false, fmt.Errorf("could not check saved ETrade configuration: %w", err)
	}
	if exists {
		answer, err := broker.AskForInput("ETrade configuration already exists. Overwrite it? [y/N]: ")
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

	if err := saveSecret(ctx, etradeBroker.Secrets, etradeConfigKey, config); err != nil {
		return false, fmt.Errorf("could not save ETrade configuration: %w", err)
	}
	return true, nil
}

func (etradeBroker Broker) Connect(ctx context.Context) (models.BrokerClient, error) {
	config, err := loadSecret[Config](ctx, etradeBroker.Secrets, etradeConfigKey)
	if errors.Is(err, secrets.ErrNotFound) {
		return nil, models.ErrBrokerNotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("%w: could not load ETrade configuration: %w", models.ErrBrokerNotConfigured, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("%w: invalid saved ETrade configuration: %w", models.ErrBrokerNotConfigured, err)
	}

	httpClient, err := authenticate(ctx, etradeBroker.Secrets, config.APIKey, config.APISecret, config.APIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", Name, err)
	}
	return &Client{httpClient: httpClient, apiBaseURL: config.APIBaseURL}, nil
}

func promptForConfig() (Config, error) {
	baseURL, err := broker.AskForInput(fmt.Sprintf("ETrade API base URL [%s]: ", productionAPIBaseURL))
	if err != nil {
		return Config{}, err
	}

	if baseURL == "" {
		baseURL = productionAPIBaseURL
	}

	apiKey, err := broker.AskForSecret("ETrade API key: ")
	if err != nil {
		return Config{}, err
	}

	apiSecret, err := broker.AskForSecret("ETrade API secret: ")
	if err != nil {
		return Config{}, err
	}

	return Config{
		APIBaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		APISecret:  apiSecret,
	}, nil
}

func loadSecret[T any](ctx context.Context, store secrets.Store, key string) (T, error) {
	var value T
	data, err := store.Get(ctx, key)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("failed to decode %s: %w", key, err)
	}
	return value, nil
}

func saveSecret(ctx context.Context, store secrets.Store, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to encode %s: %w", key, err)
	}
	return store.Set(ctx, key, data)
}
