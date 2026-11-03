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
) (Position, []LotSnapshot) {
	position := Position{
		Ticker:         ticker,
		AssetIdx:       assetIdx,
		Quote:          portfolio.Quotes[ticker],
		RecentlyBought: portfolio.RecentTx.HasBuyWithinWashWindow(ticker, createdAt),
		RecentlySold:   portfolio.RecentTx.HasSellWithinWashWindow(ticker, createdAt),
	}

	var lots []LotSnapshot
	for _, lot := range portfolio.PositionByTicker[ticker].Lots {
		if lot.Quantity.IsPositive() {
			lots = append(lots, newLotSnapshot(lot, position.Quote.Bid, createdAt, position.RecentlyBought))
		}
	}
	// Brokers list lots in no set order; sort by lot ID so plans repeat exactly.
	slices.SortFunc(lots, func(left, right LotSnapshot) int {
		return cmp.Compare(left.Id, right.Id)
	})

	return position, lots
}

func (position Position) IsHeld() bool {
	return len(position.LotIdxs) > 0
}

func (position Position) Value(lots []LotSnapshot) decimal.Decimal {
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
		lot := snapshot.Lots[idx]
		sales = append(sales, models.LotSale{
			Ticker:       position.Ticker,
			Quantity:     lot.Quantity,
			LotIdx:       idx,
			Price:        position.Quote.Bid,
			GainPerShare: lot.GainPerShare,
			GainPct:      lot.GainPct,
			LongTerm:     lot.LongTerm,
			IsWashSale:   lot.IsWashSale,
		})
	}
	return sales
}
