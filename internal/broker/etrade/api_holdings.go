package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type positionResponse struct {
	Id          int64           `json:"positionId"`
	Ticker      string          `json:"symbolDescription"`
	Quantity    decimal.Decimal `json:"quantity"`
	Allocation  decimal.Decimal `json:"pctOfPortfolio"`
	MarketValue decimal.Decimal `json:"marketValue"`
}

type positionLotResponse struct {
	Id           int64           `json:"positionLotId"`
	Quantity     decimal.Decimal `json:"remainingQty"`
	PricePaid    decimal.Decimal `json:"price"`
	MarketValue  decimal.Decimal `json:"marketValue"`
	UnrealizedPL decimal.Decimal `json:"totalGain"`
}

type portfolioResponse struct {
	PortfolioResponse struct {
		Totals struct {
			Balance     decimal.Decimal `json:"cashBalance"`
			MarketValue decimal.Decimal `json:"totalMarketValue"`
		} `json:"totals"`
		Portfolios []struct {
			Positions []positionResponse `json:"Position"`
		} `json:"AccountPortfolio"`
	} `json:"PortfolioResponse"`
}

type getPositionLotResponse struct {
	PositionLotsResponse struct {
		PositionLots []struct {
			positionLotResponse
			DateAcquired int64           `json:"acquiredDate"`
			GainPercent  decimal.Decimal `json:"totalCostForGainPct"`
		} `json:"PositionLot"`
	} `json:"PositionLotsResponse"`
}

func (client *Client) Holdings(ctx context.Context, account Account) (*models.Portfolio, error) {
	if account.key == "" {
		return nil, fmt.Errorf("expected an ETRADE account with a nonempty key")
	}

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/portfolio",
		client.apiBaseURL,
		url.PathEscape(account.key),
	)
	query := url.Values{
		"totalsRequired": {"true"},
		"view":           {"COMPLETE"},
	}
	endpoint += "?" + query.Encode()

	var timer utils.Timer
	timer.Start("Fetching holdings and cash balance")
	response, err := utils.FetchJSON[portfolioResponse](
		ctx, client.httpClient, endpoint,
	)
	timer.Stop(err)
	if err != nil {
		return nil, fmt.Errorf("fetch portfolio: %w", err)
	}

	data := response.PortfolioResponse
	if len(data.Portfolios) == 0 {
		return nil, fmt.Errorf("no portfolio returned for account %s", account.id)
	}

	portfolio := &models.Portfolio{
		BuyingPower:      data.Totals.Balance,
		TotalValue:       data.Totals.MarketValue.Add(data.Totals.Balance),
		PositionByTicker: make(map[string]models.Position),
	}

	for _, accountPortfolio := range data.Portfolios {
		for _, pos := range accountPortfolio.Positions {
			position := models.Position{
				Id:          pos.Id,
				Ticker:      strings.TrimSpace(pos.Ticker),
				Quantity:    pos.Quantity,
				Allocation:  pos.Allocation,
				MarketValue: pos.MarketValue,
			}
			timer.Start(fmt.Sprintf("Fetching tax lots for %s", position.Ticker))
			position.Lots, err = client.getPositionLots(ctx, account.key, pos.Id)
			timer.Stop(err)
			if err != nil {
				return nil, fmt.Errorf("fetch lots for position %d (%s): %w", pos.Id, pos.Ticker, err)
			}

			portfolio.PositionByTicker[position.Ticker] = position
		}
	}

	return portfolio, nil
}

func (client *Client) getPositionLots(
	ctx context.Context,
	accountKey string,
	positionID int64,
) ([]models.PositionLot, error) {
	endpoint := fmt.Sprintf(
		"%s/accounts/%s/portfolio/%d",
		client.apiBaseURL,
		url.PathEscape(accountKey),
		positionID,
	)
	response, err := utils.FetchJSON[getPositionLotResponse](
		ctx,
		client.httpClient,
		endpoint,
	)
	if err != nil {
		return nil, err
	}

	lots := make([]models.PositionLot, 0, len(response.PositionLotsResponse.PositionLots))
	for _, lot := range response.PositionLotsResponse.PositionLots {
		// ETRADE returns percentage points; the domain model uses a fraction.
		lots = append(lots, models.PositionLot{
			Id:           lot.Id,
			Quantity:     lot.Quantity,
			PricePaid:    lot.PricePaid,
			TimeAcquired: time.UnixMilli(lot.DateAcquired),
			MarketValue:  lot.MarketValue,
			UnrealizedPL: lot.UnrealizedPL,
			PLPct:        lot.GainPercent.Shift(-2),
		})
	}

	return lots, nil
}
