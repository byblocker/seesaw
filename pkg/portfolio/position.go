package portfolio

import (
	"cmp"
	"seesaw/pkg/models"
	"slices"
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

func newPosition(
	ticker string,
	assetIdx int,
	portfolio *models.Portfolio,
	createdAt time.Time,
) (Position, []models.PositionLot) {
	position := Position{
		Ticker:         ticker,
		AssetIdx:       assetIdx,
		Quote:          portfolio.Quotes[ticker],
		RecentlyBought: portfolio.RecentTx.HasBuyWithinWashWindow(ticker, createdAt),
		RecentlySold:   portfolio.RecentTx.HasSellWithinWashWindow(ticker, createdAt),
	}

	var held []models.PositionLot
	for _, lot := range portfolio.PositionByTicker[ticker].Lots {
		if lot.Quantity.IsPositive() {
			held = append(held, lot)
		}
	}
	// Brokers list lots in no set order; sort by lot ID so plans repeat exactly.
	slices.SortFunc(held, func(left, right models.PositionLot) int {
		return cmp.Compare(left.Id, right.Id)
	})

	return position, held
}

func (position Position) IsHeld() bool {
	return len(position.LotIdxs) > 0
}

func (position Position) Value(lots []models.PositionLot) decimal.Decimal {
	quantity := decimal.Zero
	for _, idx := range position.LotIdxs {
		quantity = quantity.Add(lots[idx].Quantity)
	}
	return quantity.Mul(position.Quote.Bid)
}

// Every lot of the position, sold at the bid on the snapshot's date.
func (position Position) LotSales(snapshot Snapshot) []models.LotSale {
	sales := make([]models.LotSale, 0, len(position.LotIdxs))
	for _, idx := range position.LotIdxs {
		sale := models.NewLotSale(
			position.Ticker,
			idx,
			snapshot.Lots[idx],
			position.Quote.Bid,
			snapshot.CreatedAt,
			position.RecentlyBought,
		)
		sales = append(sales, sale)
	}
	return sales
}

func LotSales(snapshot Snapshot, positionIdxs []int) []models.LotSale {
	var sales []models.LotSale
	for _, idx := range positionIdxs {
		sales = append(sales, snapshot.Positions[idx].LotSales(snapshot)...)
	}
	return sales
}
