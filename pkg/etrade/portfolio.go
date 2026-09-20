package etrade

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"seesaw/internal/httpclient"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type Portfolio struct {
	BuyingPower      decimal.Decimal
	TotalValue       decimal.Decimal
	PositionByTicker map[string]Position
}

type Position struct {
	Id          int64           `json:"positionId"`
	Ticker      string          `json:"symbolDescription"`
	Quantity    decimal.Decimal `json:"quantity"`
	Allocation  decimal.Decimal `json:"pctOfPortfolio"`
	MarketValue decimal.Decimal `json:"marketValue"`
	Lots        []PositionLot   `json:"-"`
}

type PositionLot struct {
	Id           int64           `json:"positionLotId"`
	Quantity     decimal.Decimal `json:"remainingQty"`
	PricePaid    decimal.Decimal `json:"price"`
	TimeAcquired time.Time       `json:"-"`
	MarketValue  decimal.Decimal `json:"marketValue"`
	UnrealizedPL decimal.Decimal `json:"totalGain"`
	PLPct        decimal.Decimal `json:"totalCostForGainPct"`
}

type portfolioResponse struct {
	PortfolioResponse struct {
		Totals struct {
			Balance     decimal.Decimal `json:"cashBalance"`
			MarketValue decimal.Decimal `json:"totalMarketValue"`
		} `json:"totals"`
		Portfolios []struct {
			Positions []Position `json:"Position"`
		} `json:"AccountPortfolio"`
	} `json:"PortfolioResponse"`
}

type getPositionLotResponse struct {
	PositionLotsResponse struct {
		PositionLots []struct {
			PositionLot
			DateAcquired int64 `json:"acquiredDate"`
		} `json:"PositionLot"`
	} `json:"PositionLotsResponse"`
}

func (client *Client) GetPortfolio(ctx context.Context, accountID string) (*Portfolio, error) {
	account, err := client.getAccountDetails(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("resolve account %s: %w", accountID, err)
	}

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/portfolio",
		client.apiBaseURL,
		url.PathEscape(account.Key),
	)
	response, err := httpclient.SendJSONRequest[portfolioResponse](
		ctx,
		client.httpClient,
		http.MethodGet,
		endpoint,
		nil,
		url.Values{"totalsRequired": {"true"}, "view": {"COMPLETE"}},
	)
	if err != nil {
		return nil, fmt.Errorf("fetch portfolio: %w", err)
	}

	data := response.PortfolioResponse
	if len(data.Portfolios) == 0 {
		return nil, fmt.Errorf("no portfolio returned for account %s", accountID)
	}

	positions := make(map[string]Position)
	for _, portfolio := range data.Portfolios {
		for _, pos := range portfolio.Positions {
			pos.Ticker = strings.TrimSpace(pos.Ticker)
			pos.Lots, err = client.getPositionLots(ctx, account.Key, pos.Id)
			if err != nil {
				return nil, fmt.Errorf("fetch lots for position %d (%s): %w", pos.Id, pos.Ticker, err)
			}

			positions[pos.Ticker] = pos
		}
	}

	return &Portfolio{
		BuyingPower:      data.Totals.Balance,
		TotalValue:       data.Totals.MarketValue.Add(data.Totals.Balance),
		PositionByTicker: positions,
	}, nil
}

func (client *Client) getPositionLots(ctx context.Context, accountKey string, positionID int64) ([]PositionLot, error) {
	endpoint := fmt.Sprintf(
		"%s/accounts/%s/portfolio/%d",
		client.apiBaseURL,
		url.PathEscape(accountKey),
		positionID,
	)
	response, err := httpclient.SendJSONRequest[getPositionLotResponse](
		ctx,
		client.httpClient,
		http.MethodGet,
		endpoint,
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}

	lots := make([]PositionLot, 0, len(response.PositionLotsResponse.PositionLots))
	for _, lot := range response.PositionLotsResponse.PositionLots {
		lot.TimeAcquired = time.UnixMilli(lot.DateAcquired)
		lots = append(lots, lot.PositionLot)
	}

	return lots, nil
}
