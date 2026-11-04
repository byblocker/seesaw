package liquidation

import (
	"seesaw/internal/utils"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

// Every lot with whole shares to sell, in plan order and by lot ID within each position.
func sellableLots(snapshot portfolio.Snapshot) []int {
	var lotIdxs []int
	for _, asset := range snapshot.Assets {
		for _, positionIdx := range asset.PositionIdxs {
			for _, lotIdx := range snapshot.Positions[positionIdx].LotIdxs {
				if wholeShares(snapshot.Lots[lotIdx]) > 0 {
					lotIdxs = append(lotIdxs, lotIdx)
				}
			}
		}
	}
	return lotIdxs
}

func wholeShares(lot portfolio.Lot) int64 {
	return lot.Quantity.Floor().IntPart()
}

// How much of each asset can be sold before it falls below its minimum allocation, by asset row.
func valueAboveMinAllowed(
	snapshot portfolio.Snapshot,
	maxDeviation decimal.Decimal,
	amountNeeded decimal.Decimal,
) []decimal.Decimal {
	totalValueAfterSale := snapshot.TotalValue.Sub(snapshot.Balances.CashBalance).Sub(amountNeeded)

	values := make([]decimal.Decimal, len(snapshot.Assets))
	for i, asset := range snapshot.Assets {
		minAllocAllowed := decimal.Zero
		if asset.Plan != nil {
			minAllocAllowed = utils.ClampNonNegative(asset.Plan.TargetAllocation.Sub(maxDeviation))
		}
		values[i] = asset.Value.Sub(totalValueAfterSale.Mul(minAllocAllowed))
	}
	return values
}
