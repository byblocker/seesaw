package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/broker"
	"seesaw/internal/validate"
	"seesaw/pkg/models"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type positionResponse struct {
	Id     int64  `json:"positionId" validate:"gt=0"`
	Ticker string `json:"symbolDescription" validate:"required"`
}

type positionLotResponse struct {
	Id        int64            `json:"positionLotId" validate:"gt=0"`
	Quantity  *decimal.Decimal `json:"remainingQty" validate:"required,gte=0"`
	PricePaid *decimal.Decimal `json:"price" validate:"required,gte=0"`
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
			DateAcquired int64 `json:"acquiredDate" validate:"gt=0"`
		} `json:"PositionLot"`
	} `json:"PositionLotsResponse"`
}

// List the account's positions by ticker, without their lots.
func (client *Client) listPositions(ctx context.Context, account Account) (map[string]positionResponse, error) {
	endpoint := client.endpointFor(account, "portfolio")
	query := url.Values{
		"view": {"COMPLETE"},
	}

	positionsByTicker := map[string]positionResponse{}
	currentPage, pagesFetched, totalPages := 0, 0, 0
	for {
		response, err := broker.FetchJSON[portfolioResponse](
			ctx, client.httpClient, endpoint+"?"+query.Encode(),
		)
		if err != nil {
			return nil, fmt.Errorf("fetch portfolio page %d: %w", currentPage, err)
		}

		page := response.PortfolioResponse
		if len(page.Portfolios) == 0 {
			return nil, fmt.Errorf("no portfolio returned for account %s on page %d", account.id, currentPage)
		}

		pagesFetched++
		nextPage, reportedPages, err := page.pagination()
		if err != nil {
			return nil, err
		}

		totalPages = max(totalPages, reportedPages)
		positionsBeforePage := len(positionsByTicker)
		for _, portfolio := range page.Portfolios {
			for _, position := range portfolio.Positions {
				if err := addPosition(positionsByTicker, position); err != nil {
					return nil, fmt.Errorf("load portfolio page %d: %w", currentPage, err)
				}
			}
		}

		if nextPage == 0 {
			if totalPages > pagesFetched {
				return nil, fmt.Errorf("incomplete portfolio: received %d of %d pages without a next page", pagesFetched, totalPages)
			}
			break
		}

		if nextPage <= currentPage || len(positionsByTicker) == positionsBeforePage {
			return nil, fmt.Errorf("portfolio pagination made no progress on page %d (next %d)", currentPage, nextPage)
		}

		currentPage = nextPage
		query.Set("pageNumber", strconv.Itoa(nextPage))
	}

	return positionsByTicker, nil
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

func addPosition(positionsByTicker map[string]positionResponse, position positionResponse) error {
	position.Ticker = strings.TrimSpace(position.Ticker)
	if err := validate.Fields(position, "position."); err != nil {
		return err
	}
	if _, exists := positionsByTicker[position.Ticker]; exists {
		return fmt.Errorf("duplicate portfolio ticker %q", position.Ticker)
	}
	positionsByTicker[position.Ticker] = position
	return nil
}

func (client *Client) fetchLots(
	ctx context.Context,
	account Account,
	position positionResponse,
) ([]models.TaxLot, error) {
	endpoint := client.endpointFor(account, "portfolio/"+strconv.FormatInt(position.Id, 10))
	response, err := broker.FetchJSON[getPositionLotResponse](
		ctx,
		client.httpClient,
		endpoint,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch lots for position %d (%s): %w", position.Id, position.Ticker, err)
	}

	// A position without lots would look like zero shares to the rebalancer.
	if len(response.PositionLotsResponse.PositionLots) == 0 {
		return nil, fmt.Errorf("no lots returned for position %d (%s)", position.Id, position.Ticker)
	}

	lots := make([]models.TaxLot, 0, len(response.PositionLotsResponse.PositionLots))
	for _, lot := range response.PositionLotsResponse.PositionLots {
		if err := validate.Fields(lot, "lot."); err != nil {
			return nil, fmt.Errorf("lots for position %d (%s): %w", position.Id, position.Ticker, err)
		}
		lots = append(lots, models.TaxLot{
			Ticker:       position.Ticker,
			Id:           lot.Id,
			Quantity:     *lot.Quantity,
			PricePaid:    *lot.PricePaid,
			TimeAcquired: time.UnixMilli(lot.DateAcquired),
		})
	}

	return lots, nil
}
