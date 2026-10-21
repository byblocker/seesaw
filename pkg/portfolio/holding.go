package portfolio

import (
	"fmt"
	"seesaw/pkg/models"
	"time"

	"github.com/shopspring/decimal"
)

// A held position priced at its bid, with each lot as a whole-lot sale on the sale date.
type Holding struct {
	Ticker string
	Bid    decimal.Decimal
	Value  decimal.Decimal
	Lots   []models.LotSale
}

// Price every held position at its bid, grouped in the same order as snapshot.Assets.
func (snapshot Snapshot) HoldingsByAsset(
	quotes map[string]models.Quote,
	saleDate time.Time,
) ([][]Holding, error) {
	holdingsByAsset := make([][]Holding, len(snapshot.Assets))
	for i, asset := range snapshot.Assets {
		for _, ticker := range asset.Tickers {
			position, isHeld := snapshot.Portfolio.PositionByTicker[ticker]
			if !isHeld || position.Quantity.IsZero() {
				continue
			}

			quote, quoted := quotes[ticker]
			if !quoted || !quote.Bid.IsPositive() {
				return nil, fmt.Errorf("missing bid price for %s", ticker)
			}

			isRecentlyBought := snapshot.Portfolio.RecentTx.HasBuyWithinWashWindow(ticker, saleDate)
			holding := newHolding(ticker, position, quote.Bid, saleDate, isRecentlyBought)
			holdingsByAsset[i] = append(holdingsByAsset[i], holding)
		}
	}

	return holdingsByAsset, nil
}

func newHolding(
	ticker string,
	position models.Position,
	bid decimal.Decimal,
	saleDate time.Time,
	isRecentlyBought bool,
) Holding {
	holding := Holding{Ticker: ticker, Bid: bid}
	for i := range position.Lots {
		lot := &position.Lots[i]
		if !lot.Quantity.IsPositive() {
			continue
		}

		sale := models.NewLotSale(ticker, lot, bid, saleDate, isRecentlyBought)
		holding.Lots = append(holding.Lots, sale)
		holding.Value = holding.Value.Add(sale.Proceeds())
	}

	return holding
}
