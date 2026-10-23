package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/async"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type positionResponse struct {
	Id     int64  `json:"positionId"`
	Ticker string `json:"symbolDescription"`
}

type positionLotResponse struct {
	Id        int64           `json:"positionLotId"`
	Quantity  decimal.Decimal `json:"remainingQty"`
	PricePaid decimal.Decimal `json:"price"`
}

type accountPortfolioResponse struct {
	Positions      []positionResponse `json:"Position"`
	NextPageNo     string             `json:"nextPageNo"`
	TotalNoOfPages int                `json:"totalNoOfPages"`
	TotalPages     int                `json:"totalPages"`
}

type portfolioPageResponse struct {
	Portfolios []accountPortfolioResponse `json:"AccountPortfolio"`
}

type portfolioResponse struct {
	PortfolioResponse portfolioPageResponse `json:"PortfolioResponse"`
}

type getPositionLotResponse struct {
	PositionLotsResponse struct {
		PositionLots []struct {
			positionLotResponse
			DateAcquired int64 `json:"acquiredDate"`
		} `json:"PositionLot"`
	} `json:"PositionLotsResponse"`
}

type Holdings struct {
	PositionByTicker map[string]models.Position
}

func (client *Client) Holdings(ctx context.Context, account Account) (*Holdings, error) {
	if account.key == "" {
		return nil, fmt.Errorf("expected an ETRADE account with a nonempty key")
	}

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/portfolio",
		client.apiBaseURL,
		url.PathEscape(account.key),
	)
	query := url.Values{
		"view": {"COMPLETE"},
	}

	byTicker := map[string]positionResponse{}
	currentPage, pagesFetched, totalPages := 0, 0, 0
	for {
		stop := utils.StartTimer("Fetching holdings")
		response, err := utils.FetchJSON[portfolioResponse](
			ctx, client.httpClient, endpoint+"?"+query.Encode(),
		)
		stop(err)
		if err != nil {
			return nil, fmt.Errorf("fetch portfolio page %d: %w", currentPage, err)
		}

		data := response.PortfolioResponse
		if len(data.Portfolios) == 0 {
			return nil, fmt.Errorf("no portfolio returned for account %s on page %d", account.id, currentPage)
		}

		pagesFetched++
		nextPage, reportedPages, err := data.pagination()
		if err != nil {
			return nil, err
		}

		totalPages = max(totalPages, reportedPages)
		positionsBeforePage := len(byTicker)
		if err := addPositions(byTicker, data.Portfolios); err != nil {
			return nil, fmt.Errorf("load portfolio page %d: %w", currentPage, err)
		}

		if nextPage == 0 {
			if totalPages > pagesFetched {
				return nil, fmt.Errorf("incomplete portfolio: received %d of %d pages without a next page", pagesFetched, totalPages)
			}
			break
		}

		if nextPage <= currentPage || len(byTicker) == positionsBeforePage {
			return nil, fmt.Errorf("portfolio pagination made no progress on page %d (next %d)", currentPage, nextPage)
		}

		currentPage = nextPage
		query.Set("pageNumber", strconv.Itoa(nextPage))
	}

	if len(byTicker) == 0 {
		return &Holdings{PositionByTicker: map[string]models.Position{}}, nil
	}

	stop := utils.StartTimer("Fetching tax lots for %d position(s)", len(byTicker))
	positions, err := async.ForValues(ctx, byTicker, func(ctx context.Context, pos positionResponse) (models.Position, error) {
		return client.fetchPosition(ctx, account.key, pos)
	})
	stop(err)
	if err != nil {
		return nil, err
	}
	return &Holdings{PositionByTicker: positions}, nil
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

func addPositions(byTicker map[string]positionResponse, pages []accountPortfolioResponse) error {
	for _, page := range pages {
		for _, pos := range page.Positions {
			pos.Ticker = strings.TrimSpace(pos.Ticker)
			if _, exists := byTicker[pos.Ticker]; exists {
				return fmt.Errorf("duplicate portfolio ticker %q", pos.Ticker)
			}
			byTicker[pos.Ticker] = pos
		}
	}
	return nil
}

func (client *Client) fetchPosition(
	ctx context.Context,
	accountKey string,
	pos positionResponse,
) (models.Position, error) {
	endpoint := fmt.Sprintf(
		"%s/accounts/%s/portfolio/%d",
		client.apiBaseURL,
		url.PathEscape(accountKey),
		pos.Id,
	)
	response, err := utils.FetchJSON[getPositionLotResponse](
		ctx,
		client.httpClient,
		endpoint,
	)
	if err != nil {
		return models.Position{}, fmt.Errorf("fetch lots for position %d (%s): %w", pos.Id, pos.Ticker, err)
	}

	lots := make([]models.PositionLot, 0, len(response.PositionLotsResponse.PositionLots))
	for _, lot := range response.PositionLotsResponse.PositionLots {
		lots = append(lots, models.PositionLot{
			Id:           lot.Id,
			Quantity:     lot.Quantity,
			PricePaid:    lot.PricePaid,
			TimeAcquired: time.UnixMilli(lot.DateAcquired),
		})
	}

	return models.Position{Ticker: pos.Ticker, Lots: lots}, nil
}
