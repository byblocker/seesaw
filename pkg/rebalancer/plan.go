package rebalancer

import (
	"fmt"
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

type Request struct {
	Settings config.Settings
	TaxRates *config.TaxSettings // Nil for non-taxable accounts
	Force    bool
}

func (request Request) Validate() error {
	if err := request.Settings.Validate(); err != nil {
		return err
	}
	if request.TaxRates != nil {
		return request.TaxRates.Validate()
	}
	return nil
}

func Plan(
	snapshot portfolio.Snapshot,
	request Request,
	broker models.BrokerSettings,
) (portfolio.RebalancePlan, error) {
	if err := request.Validate(); err != nil {
		return portfolio.RebalancePlan{}, err
	}
	if !snapshot.TotalValue.IsPositive() {
		return portfolio.RebalancePlan{}, fmt.Errorf("empty or negative portfolio")
	}

	request.Settings.Log(snapshot.TotalValue, broker.AllowFractionalShares)
	maxValueDeviation := snapshot.TotalValue.Mul(request.Settings.AllocationUpperLimit)
	if request.Force {
		log.Printf("  - Sell assets more than %s above target down to target", utils.FormatMoney(maxValueDeviation))
	}
	if request.TaxRates == nil {
		log.Print("  - Skip tax-loss harvesting: account is not taxable")
	}

	log.Printf("Evaluating %d asset(s) for tax-loss harvesting", len(snapshot.Assets))
	taxPerShare := portfolio.TaxPerShare(snapshot.Lots, request.TaxRates)
	var sells []portfolio.SellProposal
	var requests []buyRequest
	for _, asset := range snapshot.Assets {
		assetSells, assetRequests := rebalanceAsset(snapshot, asset, request, taxPerShare, maxValueDeviation)
		sells = append(sells, assetSells...)
		requests = append(requests, assetRequests...)
	}

	buys := sizeBuys(snapshot, request.Settings, broker, sells, requests, maxValueDeviation)
	log.Printf("Rebalance complete: %d buy(s), %d sell(s)", len(buys), len(sells))
	return portfolio.RebalancePlan{Buys: buys, Sells: sells}, nil
}

func rebalanceAsset(
	snapshot portfolio.Snapshot,
	asset portfolio.Asset,
	request Request,
	taxPerShare []decimal.Decimal,
	maxValueDeviation decimal.Decimal,
) ([]portfolio.SellProposal, []buyRequest) {
	if asset.Plan == nil {
		if request.Force {
			log.Printf("Rebalancing %s: sell all", asset.Label)
			return sellExcess(snapshot, asset, asset.Value, taxPerShare), nil
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
	var sell portfolio.SellProposal
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
		return sellExcess(snapshot, asset, asset.ValueDrift, taxPerShare), nil
	}

	buy, ok := selectBuy(snapshot, asset, sell, shortfall)
	if !ok {
		// This should rarely happen, but if we cannot maintain an asset's allocation
		// by purchasing its paired ticker, we should not harvest a ticker
		log.Printf("CANNOT buy %s: all tickers sold within wash-sale window", asset.Label)
		return nil, nil
	}

	if !sell.IsEmpty() {
		logProceeds(snapshot, sell)
	}
	log.Printf("Requesting to buy %s of %s . . .", utils.FormatMoney(buy.Amount), snapshot.Positions[buy.PositionIdx].Ticker)
	if sell.IsEmpty() {
		return nil, []buyRequest{buy}
	}
	return []portfolio.SellProposal{sell}, []buyRequest{buy}
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
	for _, idx := range asset.PositionIdxs {
		if !hasLoss(snapshot, snapshot.Positions[idx]) {
			continue
		}
		if sell := harvestPosition(snapshot, idx, settings); !sell.IsEmpty() {
			logProceeds(snapshot, sell)
			sells = append(sells, sell)
		}
	}
	return sells
}
