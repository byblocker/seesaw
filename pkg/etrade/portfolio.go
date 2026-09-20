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
	PLPct        decimal.Decimal `json:"-"`
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
			DateAcquired int64           `json:"acquiredDate"`
			GainPercent  decimal.Decimal `json:"totalCostForGainPct"`
		} `json:"PositionLot"`
	} `json:"PositionLotsResponse"`
}

func (client *Client) GetPortfolio(ctx context.Context, account *AccountDetails) (*Portfolio, error) {
	if account == nil || account.Key == "" {
		return nil, fmt.Errorf("invalid account details")
	}

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/portfolio",
		client.apiBaseURL,
		url.PathEscape(account.Key),
	)
	query := url.Values{
		"totalsRequired": {"true"},
		"view":           {"COMPLETE"},
	}

]	response, err := httpclient.SendJSONRequest[portfolioResponse](
		ctx, client.httpClient, http.MethodGet, endpoint, nil, query,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch portfolio: %w", err)
	}

	data := response.PortfolioResponse
	if len(data.Portfolios) == 0 {
		return nil, fmt.Errorf("no portfolio returned for account %s", account.Id)
	}

	portfolio := &Portfolio{
		BuyingPower:      data.Totals.Balance,
		TotalValue:       data.Totals.MarketValue.Add(data.Totals.Balance),
		PositionByTicker: make(map[string]Position),
	}

	for _, accountPortfolio := range data.Portfolios {
		for _, pos := range accountPortfolio.Positions {
			pos.Ticker = strings.TrimSpace(pos.Ticker)
			pos.Lots, err = client.getPositionLots(ctx, account.Key, pos.Id)
			if err != nil {
				return nil, fmt.Errorf("fetch lots for position %d (%s): %w", pos.Id, pos.Ticker, err)
			}

			portfolio.PositionByTicker[pos.Ticker] = pos
		}
	}

	return portfolio, nil
}

func (client *Client) getPositionLots(
	ctx context.Context,
	accountKey string,
	positionID int64,
) ([]PositionLot, error) {
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

		// Despite its name, totalCostForGainPct contains percentage points in
		// broker responses. Convert to a fraction: -0.5% becomes -0.005.
		lot.PLPct = lot.GainPercent.Shift(-2)
		lots = append(lots, lot.PositionLot)
	}

	return lots, nil
}
