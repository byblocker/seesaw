package portfolio

import (
	"fmt"
	"seesaw/internal/calendar"
	"seesaw/pkg/models"
	"time"

	"github.com/shopspring/decimal"
)

type Position struct {
	Ticker         string
	AssetIdx       int
	LotIdxs        []int
	Quote          models.Quote
	RecentlyBought bool
	RecentlySold   bool
}

// Write each quote into its position, and reject any position left without a positive bid and ask.
func setQuotes(positions []Position, positionIdxByTicker map[string]int, quotes []models.Quote) error {
	for _, quote := range quotes {
		if idx, ok := positionIdxByTicker[quote.Ticker]; ok {
			positions[idx].Quote = quote
		}
	}
	for _, position := range positions {
		if !position.Quote.Bid.IsPositive() || !position.Quote.Ask.IsPositive() {
			return fmt.Errorf("missing quote for %s", position.Ticker)
		}
	}
	return nil
}

// Trades within the wash-sale window bar harvesting a recent buy and buying back a recent sale.
func flagRecentTrades(positions []Position, positionIdxByTicker map[string]int, transactions []models.Transaction, createdAt time.Time) {
	for _, tx := range transactions {
		idx, ok := positionIdxByTicker[tx.Ticker]
		if !ok || calendar.DaysBetween(tx.Date, createdAt) > washSaleWindowDays {
			continue
		}
		switch tx.Type {
		case models.TxBuy:
			positions[idx].RecentlyBought = true
		case models.TxSell:
			positions[idx].RecentlySold = true
		}
	}
}

func (position Position) Value(lots []LotSnapshot) decimal.Decimal {
	quantity := decimal.Zero
	for _, idx := range position.LotIdxs {
		quantity = quantity.Add(lots[idx].Quantity)
	}
	return quantity.Mul(position.Quote.Bid)
}
