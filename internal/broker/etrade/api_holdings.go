package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"strconv"
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

type accountPortfolioResponse struct {
	Positions      []positionResponse `json:"Position"`
	NextPageNo     string             `json:"nextPageNo"`
	TotalNoOfPages int                `json:"totalNoOfPages"`
	TotalPages     int                `json:"totalPages"`
}

type portfolioPageResponse struct {
	Totals struct {
		Balance     decimal.Decimal `json:"cashBalance"`
		MarketValue decimal.Decimal `json:"totalMarketValue"`
	} `json:"totals"`
	Portfolios []accountPortfolioResponse `json:"AccountPortfolio"`
}

type portfolioResponse struct {
	PortfolioResponse portfolioPageResponse `json:"PortfolioResponse"`
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

	portfolio := &models.Portfolio{
		PositionByTicker: make(map[string]models.Position),
	}
	var timer utils.Timer
	currentPage, pagesFetched, totalPages := 0, 0, 0
	for {
		timer.Start("Fetching holdings and cash balance")
		response, err := utils.FetchJSON[portfolioResponse](
			ctx, client.httpClient, endpoint+"?"+query.Encode(),
		)
		timer.Stop(err)
		if err != nil {
			return nil, fmt.Errorf("fetch portfolio page %d: %w", currentPage, err)
		}

		data := response.PortfolioResponse
		if len(data.Portfolios) == 0 {
			return nil, fmt.Errorf("no portfolio returned for account %s on page %d", account.id, currentPage)
		}
		if pagesFetched == 0 {
			// Totals describe the account, not each page's positions.
			portfolio.BuyingPower = data.Totals.Balance
			portfolio.TotalValue = data.Totals.MarketValue.Add(data.Totals.Balance)
		}

		pagesFetched++
		nextPage, reportedPages, err := data.pagination()
		if err != nil {
			return nil, err
		}

		totalPages = max(totalPages, reportedPages)
		positionsBeforePage := len(portfolio.PositionByTicker)
		if err := client.parsePositions(ctx, account.key, data.Portfolios, portfolio.PositionByTicker); err != nil {
			return nil, fmt.Errorf("load portfolio page %d: %w", currentPage, err)
		}

		if nextPage == 0 {
			if totalPages > pagesFetched {
				return nil, fmt.Errorf("incomplete portfolio: received %d of %d pages without a next page", pagesFetched, totalPages)
			}
			break
		}

		if nextPage <= currentPage || len(portfolio.PositionByTicker) == positionsBeforePage {
			return nil, fmt.Errorf("portfolio pagination made no progress on page %d (next %d)", currentPage, nextPage)
		}

		currentPage = nextPage
		query.Set("pageNumber", strconv.Itoa(nextPage))
	}

	return portfolio, nil
}

func (page portfolioPageResponse) pagination() (nextPage, totalPages int, err error) {
	for _, accountPortfolio := range page.Portfolios {
		totalPages = max(totalPages, accountPortfolio.TotalNoOfPages, accountPortfolio.TotalPages)
		if accountPortfolio.NextPageNo == "" {
			continue
		}
		pageNumber, err := strconv.Atoi(accountPortfolio.NextPageNo)
		if err != nil || pageNumber < 0 {
			return 0, 0, fmt.Errorf("invalid portfolio next page %q", accountPortfolio.NextPageNo)
		}

		if nextPage != 0 && pageNumber != 0 && nextPage != pageNumber {
			return 0, 0, fmt.Errorf("conflicting portfolio next pages %d and %d", nextPage, pageNumber)
		}

		nextPage = max(nextPage, pageNumber)
	}

	return nextPage, totalPages, nil
}

func (client *Client) parsePositions(
	ctx context.Context,
	accountKey string,
	pages []accountPortfolioResponse,
	positions map[string]models.Position,
) error {
	var timer utils.Timer
	for _, page := range pages {
		for _, pos := range page.Positions {
			position := models.Position{
				Id:          pos.Id,
				Ticker:      strings.TrimSpace(pos.Ticker),
				Quantity:    pos.Quantity,
				Allocation:  pos.Allocation,
				MarketValue: pos.MarketValue,
			}
			if _, exists := positions[position.Ticker]; exists {
				return fmt.Errorf("duplicate portfolio ticker %q", position.Ticker)
			}
			timer.Start(fmt.Sprintf("Fetching tax lots for %s", position.Ticker))
			lots, err := client.getPositionLots(ctx, accountKey, pos.Id)
			timer.Stop(err)
			if err != nil {
				return fmt.Errorf("fetch lots for position %d (%s): %w", pos.Id, pos.Ticker, err)
			}
			position.Lots = lots
			positions[position.Ticker] = position
		}
	}
	return nil
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
