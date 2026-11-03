package rebalancer

import (
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

func Plan(
	snapshot portfolio.Snapshot,
	request models.RebalanceRequest,
	broker models.BrokerSettings,
) (*models.RebalancePlan, error) {
	request.Settings.Log(snapshot.TotalValue, broker.AllowFractionalShares)
	maxValueDeviation := snapshot.TotalValue.Mul(request.Settings.AllocationUpperLimit)
	if request.Force {
		log.Printf("  - Sell assets more than %s above target down to target", utils.FormatMoney(maxValueDeviation))
	}
	if request.TaxRates == nil {
		log.Print("  - Skip tax-loss harvesting: account is not taxable")
	}

	log.Printf("Evaluating %d asset(s) for tax-loss harvesting", len(snapshot.Assets))
	var sells []models.SellProposal
	var requests []buyRequest
	for _, asset := range snapshot.Assets {
		assetSells, assetRequests := rebalanceAsset(snapshot, asset, request, maxValueDeviation)
		sells = append(sells, assetSells...)
		requests = append(requests, assetRequests...)
	}

	buys, err := sizeBuys(snapshot, request.Settings, broker, sells, requests, maxValueDeviation)
	if err != nil {
		return nil, err
	}
	return &models.RebalancePlan{Buys: buys, Sells: sells}, nil
}

func rebalanceAsset(
	snapshot portfolio.Snapshot,
	asset portfolio.Asset,
	request models.RebalanceRequest,
	maxValueDeviation decimal.Decimal,
) ([]models.SellProposal, []buyRequest) {
	if asset.Plan == nil {
		if request.Force {
			log.Printf("Rebalancing %s: sell all", asset.Label)
			return sellExcess(snapshot, asset, asset.Value, request.TaxRates), nil
		}
		if request.TaxRates == nil {
			log.Printf("SKIP rebalancing %s: outside the plan", asset.Label)
			return nil, nil
		}
		log.Printf("Rebalancing %s: harvest only", asset.Label)
		return harvestUnplanned(snapshot, asset, request.Settings), nil
	}

	log.Printf("Rebalancing %s: %s --> %s",
		asset.Label, utils.FormatPercent(asset.Allocation), utils.FormatPercent(asset.Plan.TargetAllocation))
	for _, idx := range asset.PositionIdxs {
		if position := snapshot.Positions[idx]; position.RecentlySold {
			log.Printf("CANNOT buy %s: sold within wash-sale window", position.Ticker)
		}
	}

	// Harvesting does nothing for an account that is not taxable, and its buys could wash a taxable account's loss.
	var sell models.SellProposal
	if request.TaxRates != nil {
		sell = harvestAsset(snapshot, asset, request.Settings)
	}
	shortfall := sell.Proceeds.Sub(asset.ValueDrift)
	if !shortfall.IsPositive() {
		// We should not buy into an overweight asset, so its sale cannot be replaced either.
		log.Printf("CANNOT buy %s: position is overweight", asset.Label)
		if !request.Force {
			return nil, nil
		}
		if asset.ValueDrift.LessThanOrEqual(maxValueDeviation) {
			log.Printf("SKIP selling %s: within %s of target", asset.Label, utils.FormatMoney(maxValueDeviation))
			return nil, nil
		}
		return sellExcess(snapshot, asset, asset.ValueDrift, request.TaxRates), nil
	}

	buy, ok := selectBuy(snapshot, asset, sell, shortfall)
	if !ok {
		// This should rarely happen, but if we cannot maintain an asset's allocation
		// by purchasing its paired ticker, we should not harvest a ticker
		log.Printf("CANNOT buy %s: all tickers sold within wash-sale window", asset.Label)
		return nil, nil
	}

	if !sell.IsEmpty() {
		sell.LogProceeds()
	}
	log.Printf("Requesting to buy %s of %s . . .", utils.FormatMoney(buy.Amount), buy.Ticker)
	if sell.IsEmpty() {
		return nil, []buyRequest{buy}
	}
	return []models.SellProposal{sell}, []buyRequest{buy}
}

// Unless forced, we should not liquidate unplanned positions while rebalancing,
// but we should eagerly harvest any tax-loss these positions have accrued.
func harvestUnplanned(
	snapshot portfolio.Snapshot,
	asset portfolio.Asset,
	settings config.Settings,
) []models.SellProposal {
	if !assetHasLoss(snapshot, asset) {
		log.Printf("CANNOT harvest %s: no lots with loss", asset.Label)
		return nil
	}

	var sells []models.SellProposal
	for _, idx := range asset.PositionIdxs {
		position := snapshot.Positions[idx]
		if !hasLoss(snapshot, position) {
			continue
		}
		if sell := harvestPosition(snapshot, position, settings); !sell.IsEmpty() {
			sell.LogProceeds()
			sells = append(sells, sell)
		}
	}
	return sells
}
