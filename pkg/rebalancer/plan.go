package rebalancer

import (
	"fmt"
	"log"
	"seesaw/internal/format"
	"seesaw/pkg/config"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

func Plan(
	snapshot portfolio.Snapshot,
	settings config.Settings,
	force bool,
) (portfolio.Proposal, error) {
	if err := settings.Validate(); err != nil {
		return portfolio.Proposal{}, err
	}
	if !snapshot.TotalValue.IsPositive() {
		return portfolio.Proposal{}, fmt.Errorf("empty or negative portfolio")
	}

	maxValueDeviation := snapshot.TotalValue.Mul(settings.AllocationUpperLimit)
	logSettings(snapshot, settings, force, maxValueDeviation)

	log.Printf("Evaluating %d asset(s) for tax-loss harvesting", len(snapshot.Assets))
	var sells []portfolio.SellProposal
	var buyRequests []buyRequest
	for _, asset := range snapshot.Assets {
		assetSells, assetBuyRequests := rebalanceAsset(snapshot, asset, settings, force, maxValueDeviation)
		sells = append(sells, assetSells...)
		buyRequests = append(buyRequests, assetBuyRequests...)
	}

	buys := sizeBuys(snapshot, settings, sells, buyRequests, maxValueDeviation)
	log.Printf("Rebalance complete: %d buy(s), %d sell(s)", len(buys), len(sells))
	return portfolio.Proposal{Sells: sells, Buys: buys}, nil
}

func logSettings(snapshot portfolio.Snapshot, settings config.Settings, force bool, maxValueDeviation decimal.Decimal) {
	log.Print("Rebalance settings:")
	log.Printf("  - Harvest losses of %s or more", format.Percent(settings.MinPctLossToHarvest.Neg()))
	if snapshot.Broker.AllowFractionalShares {
		log.Print("  - Buy fractional shares")
	} else {
		log.Printf("  - Round up from %s share", settings.MinFractionalShareToRound)
		log.Printf("  - Allow at most %s above target (~%s)", format.Percent(settings.AllocationUpperLimit), format.Money(maxValueDeviation))
	}
	if force {
		log.Printf("  - Sell assets more than %s above target down to target", format.Money(maxValueDeviation))
	}
	if snapshot.TaxRates == nil {
		log.Print("  - Skip tax-loss harvesting: account is not taxable")
	}
}

func rebalanceAsset(
	snapshot portfolio.Snapshot,
	asset portfolio.Asset,
	settings config.Settings,
	force bool,
	maxValueDeviation decimal.Decimal,
) ([]portfolio.SellProposal, []buyRequest) {
	if asset.Plan == nil {
		if force {
			log.Printf("Rebalancing %s: sell all", asset.Label)
			return sellExcess(snapshot, asset, asset.Value), nil
		}
		if snapshot.TaxRates == nil {
			log.Printf("SKIP rebalancing %s: outside the plan", asset.Label)
			return nil, nil
		}
		log.Printf("Rebalancing %s: harvest only", asset.Label)
		return harvestUnplanned(snapshot, asset, settings), nil
	}

	log.Printf("Rebalancing %s: %s --> %s",
		asset.Label, format.Percent(asset.Allocation), format.Percent(asset.Plan.TargetAllocation))
	for _, positionIdx := range asset.PositionIdxs {
		if position := snapshot.Positions[positionIdx]; position.RecentlySold {
			log.Printf("CANNOT buy %s: sold within wash-sale window", position.Ticker)
		}
	}

	// Harvesting does nothing for an account that is not taxable, and its buys could wash a taxable account's loss.
	var sell portfolio.SellProposal
	if snapshot.TaxRates != nil {
		sell = harvestAsset(snapshot, asset, settings)
	}
	shortfall := sell.Proceeds.Sub(asset.ValueDrift)
	if !shortfall.IsPositive() {
		// We should not buy into an overweight asset, so its sale cannot be replaced either.
		log.Printf("CANNOT buy %s: position is overweight", asset.Label)
		if !force {
			return nil, nil
		}
		if asset.ValueDrift.LessThanOrEqual(maxValueDeviation) {
			log.Printf("SKIP selling %s: within %s of target", asset.Label, format.Money(maxValueDeviation))
			return nil, nil
		}
		return sellExcess(snapshot, asset, asset.ValueDrift), nil
	}

	soldPositionIdx := -1
	if !sell.IsEmpty() {
		soldPositionIdx = sell.PositionIdx
	}
	buyPositionIdx, ok := replacement(snapshot, asset, soldPositionIdx)
	if !ok {
		// This can happen when all positions are recently sold and cannot be replaced.
		log.Printf("CANNOT buy %s: replacement ticker(s) sold within wash-sale window", asset.Label)
		return nil, nil
	}

	if !sell.IsEmpty() {
		logProceeds(snapshot, sell)
	}
	log.Printf("Requesting to buy %s of %s . . .", format.Money(shortfall), snapshot.Positions[buyPositionIdx].Ticker)
	toBuy := []buyRequest{{PositionIdx: buyPositionIdx, Amount: shortfall}}
	if sell.IsEmpty() {
		return nil, toBuy
	}
	return []portfolio.SellProposal{sell}, toBuy
}

// Unless forced, we should not liquidate unplanned positions while rebalancing,
// but we should eagerly harvest any tax-loss these positions have accrued.
func harvestUnplanned(
	snapshot portfolio.Snapshot,
	asset portfolio.Asset,
	settings config.Settings,
) []portfolio.SellProposal {
	if !assetHasLoss(snapshot, asset) {
		log.Printf("CANNOT harvest %s: no lots with loss", asset.Label)
		return nil
	}

	var sells []portfolio.SellProposal
	for _, positionIdx := range asset.PositionIdxs {
		if !hasLoss(snapshot, snapshot.Positions[positionIdx]) {
			continue
		}
		if sell := harvestPosition(snapshot, positionIdx, settings); !sell.IsEmpty() {
			logProceeds(snapshot, sell)
			sells = append(sells, sell)
		}
	}
	return sells
}
