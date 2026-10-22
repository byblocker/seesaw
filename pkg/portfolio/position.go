package portfolio

import (
	"seesaw/pkg/models"
	"time"

	"github.com/shopspring/decimal"
)

type Position struct {
	Ticker         string
	Lots           []models.PositionLot
	Quote          models.Quote
	RecentlyBought bool
	RecentlySold   bool
}

func newPosition(
	ticker string,
	portfolio *models.Portfolio,
	quote models.Quote,
	createdAt time.Time,
) Position {
	position := Position{
		Ticker:         ticker,
		Quote:          quote,
		RecentlyBought: portfolio.RecentTx.HasBuyWithinWashWindow(ticker, createdAt),
		RecentlySold:   portfolio.RecentTx.HasSellWithinWashWindow(ticker, createdAt),
	}

	for _, lot := range portfolio.PositionByTicker[ticker].Lots {
		if lot.Quantity.IsPositive() {
			position.Lots = append(position.Lots, lot)
		}
	}

	return position
}

func (position Position) IsHeld() bool {
	return len(position.Lots) > 0
}

func (position Position) Quantity() decimal.Decimal {
	quantity := decimal.Zero
	for _, lot := range position.Lots {
		quantity = quantity.Add(lot.Quantity)
	}
	return quantity
}

func (position Position) Value() decimal.Decimal {
	return position.Quantity().Mul(position.Quote.Bid)
}

func (position Position) LotSales(saleDate time.Time) []models.LotSale {
	sales := make([]models.LotSale, 0, len(position.Lots))
	for i := range position.Lots {
		sale := models.NewLotSale(
			position.Ticker,
			&position.Lots[i],
			position.Quote.Bid,
			saleDate,
			position.RecentlyBought,
		)
		sales = append(sales, sale)
	}
	return sales
}
