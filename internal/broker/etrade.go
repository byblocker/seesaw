package broker

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"seesaw/internal/broker/etrade"
	"seesaw/internal/collections"
	"seesaw/pkg/models"
	"slices"
	"strings"
	"time"
)

const ETRADE = "ETRADE"

type ETradeConfig struct {
	APIBaseURL string
	APIKey     string
	APISecret  string
}

type ETradeClient struct {
	api *etrade.Client
}

var _ models.Broker = (*ETradeClient)(nil)

func NewETradeClient(ctx context.Context, config ETradeConfig) (*ETradeClient, error) {
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("ETrade APIKey must not be empty")
	}
	if strings.TrimSpace(config.APISecret) == "" {
		return nil, fmt.Errorf("ETrade APISecret must not be empty")
	}
	baseURL, err := url.Parse(config.APIBaseURL)
	if err != nil || baseURL.Hostname() == "" || (baseURL.Scheme != "https" && baseURL.Scheme != "http") {
		return nil, fmt.Errorf("ETrade APIBaseURL must be an absolute HTTP or HTTPS URL")
	}
	if baseURL.User != nil || baseURL.RawQuery != "" || baseURL.ForceQuery || baseURL.Fragment != "" {
		return nil, fmt.Errorf("ETrade APIBaseURL must not contain user information, a query, or a fragment")
	}

	client, err := etrade.Connect(ctx, config.APIKey, config.APISecret, strings.TrimRight(config.APIBaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ETRADE: %w", err)
	}
	return &ETradeClient{api: client}, nil
}

func (client *ETradeClient) Settings() models.BrokerSettings {
	return models.BrokerSettings{AllowFractionalShares: false}
}

func (client *ETradeClient) Quote(ctx context.Context, ticker string) (models.Quote, error) {
	return client.api.Quote(ctx, ticker)
}

func (client *ETradeClient) Portfolio(ctx context.Context, accountID string, daysBack int, tickers []string) (*models.Portfolio, error) {
	if daysBack <= 0 {
		return nil, fmt.Errorf("transaction lookback must be positive")
	}

	accounts, err := client.api.Accounts(ctx)
	if err != nil {
		return nil, err
	}

	var selected *etrade.Account
	for i := range accounts {
		if accounts[i].ID() == accountID {
			selected = &accounts[i]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("could not find active brokerage account %s", accountID)
	}

	portfolio, err := client.api.Holdings(ctx, *selected)
	if err != nil {
		return nil, err
	}

	portfolio.Tickers = collections.SetFrom(slices.Values(tickers), maps.Keys(portfolio.PositionByTicker)).Slice()
	end := time.Now()
	start := end.AddDate(0, 0, -daysBack)
	portfolio.RecentTx = make(models.RecentTxByTicker)
	for _, account := range accounts {
		history, err := client.api.Transactions(ctx, account, start, end, portfolio.Tickers)
		if err != nil {
			return nil, fmt.Errorf("could not get transactions for account %s: %w", account.ID(), err)
		}
		portfolio.RecentTx.Append(history)
	}
	return portfolio, nil
}
