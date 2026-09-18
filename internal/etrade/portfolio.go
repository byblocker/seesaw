package etrade

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"seesaw/internal"
	"strings"
	"time"

	"github.com/quagmt/udecimal"
)

type Portfolio struct {
	BuyingPower      udecimal.Decimal
	TotalValue       udecimal.Decimal
	PositionByTicker map[string]Position
}

type viewPortfolioResponse struct {
	PortfolioResponse struct {
		Totals struct {
			Balance     udecimal.Decimal `json:"cashBalance"`
			MarketValue udecimal.Decimal `json:"totalMarketValue"`
		} `json:"totals"`
		Portfolios []struct {
			AccountId string `json:"accountId"`
			Positions []struct {
				Id       int64   `json:"positionId"`
				Ticker   string  `json:"symbolDescription"`
				Quantity float64 `json:"quantity"`

				Allocation  udecimal.Decimal `json:"pctOfPortfolio"`
				MarketValue udecimal.Decimal `json:"marketValue"`
				Complete    struct {
					Ask udecimal.Decimal `json:"ask"`
					Bid udecimal.Decimal `json:"bid"`
				} `json:"Complete"`
			} `json:"Position"`
		} `json:"AccountPortfolio"`
	} `json:"PortfolioResponse"`
}

type Position struct {
	Id       int64
	Ticker   string
	Quantity float64

	Ask         udecimal.Decimal
	Bid         udecimal.Decimal
	Allocation  udecimal.Decimal
	MarketValue udecimal.Decimal

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

func (client Client) GetPortfolio(ctx context.Context, account *AccountDetails) (*Portfolio, error) {
	log.Print("Fetching portfolio . . .")

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/portfolio",
		client.APIBaseUrl,
		account.Key,
	)

	response, err := internal.SendJSONRequest[viewPortfolioResponse](
		ctx,
		client.httpClient,
		http.MethodGet,
		endpoint,
		nil,
		url.Values{
			"totalsRequired": {"true"},
			"view":           {"COMPLETE"},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch positions: %w", err)
	}

	if len(response.PortfolioResponse.Portfolios) == 0 {
		return nil, fmt.Errorf("no portfolio returned for account %s", account.Id)
	}

	byTicker := make(map[string]Position)

	positions := response.PortfolioResponse.Portfolios[0].Positions
	for _, pos := range positions {
		pos.Ticker = strings.TrimSpace(pos.Ticker)
		log.Printf("Fetching lots for %s...", pos.Ticker)

		lots, err := client.fetchLotsForPosition(ctx, account.Key, pos.Id)
		if err != nil {
			return nil, err
		}

		byTicker[pos.Ticker] = Position{
			Id:          pos.Id,
			Ticker:      pos.Ticker,
			Quantity:    pos.Quantity,
			Ask:         pos.Complete.Ask,
			Bid:         pos.Complete.Bid,
			Allocation:  pos.Allocation,
			MarketValue: pos.MarketValue,
			Lots:        lots,
		}
	}

	return &Portfolio{
		BuyingPower:      response.PortfolioResponse.Totals.Balance,
		TotalValue:       response.PortfolioResponse.Totals.MarketValue.Add(response.PortfolioResponse.Totals.Balance),
		PositionByTicker: byTicker,
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
		lots[i].TimeAcquired = convertUnixToTime(lots[i].DateAcquired)
	}

	return response.PositionLotsResponse.PositionLots, nil
}

func convertUnixToTime(timestamp int) time.Time {
	// ETrade encodes time as a Unix timestamp in milliseconds
	return time.Unix(int64(timestamp/1000), 0)
}
