package portfolio

import (
	"cmp"
	"fmt"
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
	quote models.Quote,
	heldLots []models.Lot,
	recentlyBought, recentlySold bool,
	createdAt time.Time,
) (Position, []LotSnapshot, error) {
	if !quote.Bid.IsPositive() || !quote.Ask.IsPositive() {
		return Position{}, nil, fmt.Errorf("missing quote for %s", ticker)
	}

	position := Position{
		Ticker:         ticker,
		AssetIdx:       assetIdx,
		Quote:          quote,
		RecentlyBought: recentlyBought,
		RecentlySold:   recentlySold,
	}

	var lots []LotSnapshot
	for _, lot := range heldLots {
		if lot.Quantity.IsPositive() {
			lots = append(lots, newLotSnapshot(lot, quote.Bid, createdAt, position.RecentlyBought))
		}
	}

	// Brokers list lots in no set order; sort by lot ID so plans repeat exactly.
	slices.SortFunc(lots, func(left, right LotSnapshot) int {
		return cmp.Compare(left.Id, right.Id)
	})

	return position, lots, nil
}

func (position Position) Value(lots []LotSnapshot) decimal.Decimal {
	quantity := decimal.Zero
	for _, idx := range position.LotIdxs {
		quantity = quantity.Add(lots[idx].Quantity)
	}
	return quantity.Mul(position.Quote.Bid)
}
