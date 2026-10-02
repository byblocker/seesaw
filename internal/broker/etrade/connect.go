package etrade

import (
	"context"
	"errors"
	"fmt"
	"log"
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

func NewClient(ctx context.Context, config Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	baseURL := strings.TrimRight(config.APIBaseURL, "/")
	log.Printf("Connecting to %s...", Name)
	httpClient, err := authenticate(ctx, config.APIKey, config.APISecret, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", Name, err)
	}

	return &Client{httpClient: httpClient, apiBaseURL: baseURL}, nil
}

type Broker struct{}

func (Broker) Setup(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	exists, err := keyring.Exists(etradeConfigKey)
	if err != nil {
		return fmt.Errorf("could not check saved ETrade configuration; check OS keyring access: %w", err)
	}
	if exists {
		answer, err := utils.AskForInput("ETrade configuration already exists. Overwrite it? [y/N]: ")
		if err != nil {
			return err
		}
		if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
			return context.Canceled
		}
	}

	config, err := promptForConfig()
	if err != nil {
		return err
	}

	if err := config.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := keyring.Set(etradeConfigKey, config); err != nil {
		return fmt.Errorf("could not save ETrade configuration; check OS keyring access: %w", err)
	}
	return nil
}

func (Broker) Connect(ctx context.Context) (models.BrokerClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	config, err := keyring.Get[Config](etradeConfigKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("%w: run seesaw connect --broker ETRADE", models.ErrBrokerNotConfigured)
	}
	if err != nil {
		return nil, fmt.Errorf("could not load ETrade configuration; check OS keyring access or run seesaw connect --broker ETRADE: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid saved ETrade configuration; run seesaw connect --broker ETRADE: %w", err)
	}

	return NewClient(ctx, *config)
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
