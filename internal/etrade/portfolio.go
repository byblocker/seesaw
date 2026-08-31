package etrade

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"seesaw/internal"
	"time"

	"github.com/quagmt/udecimal"
)

type Portfolio struct {
	BuyingPower udecimal.Decimal
	Positions   []Position
}

type viewPortfolioResponse struct {
	PortfolioResponse struct {
		Totals struct {
			Balance udecimal.Decimal `json:"cashBalance"`
		} `json:"totals"`
		Portfolios []struct {
			AccountId string     `json:"accountId"`
			Positions []Position `json:"Position"`
		} `json:"AccountPortfolio"`
	} `json:"PortfolioResponse"`
}

type Position struct {
	Id         int64            `json:"positionId"`
	Ticker     string           `json:"symbolDescription"`
	Quantity   float64          `json:"quantity"`
	Allocation udecimal.Decimal `json:"pctOfPortfolio"`

	Lots []PositionLot
}

type getPositionLotResponse struct {
	PositionLotsResponse struct {
		PositionLots []PositionLot `json:"PositionLot"`
	} `json:"PositionLotsResponse"`
}

type PositionLot struct {
	Id           int64   `json:"positionLotId"`
	Quantity     float64 `json:"remainingQty"`
	PricePaid    float64 `json:"price"`
	DateAcquired int     `json:"acquiredDate"`
	TimeAcquired time.Time

	MarketValue  udecimal.Decimal `json:"marketValue"`
	UnrealizedPL udecimal.Decimal `json:"totalGain"`
	PLPct        udecimal.Decimal `json:"totalCostForGainPct"`
}

func (client Client) GetPortfolio(ctx context.Context, accountKey string) (*Portfolio, error) {
	log.Print("Fetching portfolio . . .")

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/portfolio",
		client.APIBaseUrl,
		accountKey,
	)

	response, err := internal.SendJSONRequest[viewPortfolioResponse](
		ctx,
		client.httpClient,
		http.MethodGet,
		endpoint,
		nil,
		url.Values{
			"totalsRequired": {"true"},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch positions: %w", err)
	}

	portfolios := response.PortfolioResponse.Portfolios
	if len(portfolios) == 0 {
		return nil, fmt.Errorf("no portfolio returned for account %s", accountKey)
	}

	positions := portfolios[0].Positions

	for i := range positions {
		log.Printf("Fetching lots for %s . . .", positions[i].Ticker)
		lots, err := client.fetchLotsForPosition(ctx, accountKey, positions[i].Id)
		if err != nil {
			return nil, err
		}

		positions[i].Lots = lots
	}

	return &Portfolio{
		BuyingPower: response.PortfolioResponse.Totals.Balance,
		Positions:   positions,
	}, nil
}

func (client Client) fetchLotsForPosition(
	ctx context.Context,
	accountKey string,
	positionId int64,
) ([]PositionLot, error) {
	endpoint := fmt.Sprintf("%s/accounts/%s/portfolio/%d",
		client.APIBaseUrl,
		accountKey,
		positionId,
	)

	response, err := internal.SendJSONRequest[getPositionLotResponse](
		ctx,
		client.httpClient,
		http.MethodGet,
		endpoint,
		nil,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch lots for %d: %w", positionId, err)
	}

	lots := response.PositionLotsResponse.PositionLots

	for i := range lots {
		// ETrade encodes time as a Unix timestamp in milliseconds
		lots[i].TimeAcquired = time.Unix(int64(lots[i].DateAcquired/1000), 0)
	}

	return response.PositionLotsResponse.PositionLots, nil
}
