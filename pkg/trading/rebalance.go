package trading

import (
	"context"
	"fmt"
	"log"
	"seesaw/internal/collections"
	"seesaw/internal/rebalance"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

type RebalancePlan struct {
	Buys  []models.Buy
	Sells []models.SellProposal
}

func Rebalance(
	ctx context.Context,
	snapshot portfolio.Snapshot,
	client models.BrokerClient,
	settings config.RebalanceSettings,
) (*RebalancePlan, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	if snapshot.Portfolio == nil {
		return nil, fmt.Errorf("snapshot is missing portfolio")
	}
	if snapshot.Portfolio.TotalValue.Cmp(decimal.Zero) <= 0 {
		return nil, fmt.Errorf("empty or negative portfolio")
	}

	var (
		buyRequests []rebalance.BuyRequest
		plan        RebalancePlan
	)

	saleDate := time.Now()
	brokerSettings := client.Settings()

	balance := snapshot.Portfolio.Balances.BuyingPower
	maxValueDeviationAllowed := snapshot.Portfolio.TotalValue.Mul(
		*settings.MaxPctAlocDeviation,
	)

	log.Printf("Fetching quotes for %d ticker(s)", len(snapshot.Portfolio.AllTickers))
	quotePerTicker, err := client.QuoteAll(ctx, snapshot.Portfolio.AllTickers)
	if err != nil {
		return nil, err
	}

	holdingsByAsset, err := snapshot.HoldingsByAsset(quotePerTicker, saleDate)
	if err != nil {
		return nil, err
	}

	log.Printf("Evaluating %d asset(s) for tax-loss harvesting", len(snapshot.Assets))
	for i, asset := range snapshot.Assets {
		if asset.Plan != nil {
			utils.Verbosef(ctx, "Evaluating %s: current %s%%, target %s%%",
				asset.Label(),
				asset.Allocation.Shift(2).StringFixed(2), asset.Plan.TargetAllocation.Shift(2).StringFixed(2))
			for ticker := range asset.Plan.Tickers() {
				if snapshot.Portfolio.RecentTx.HasRecentSell(ticker) {
					utils.Verbosef(ctx, "Skip buying %s: recent sales in the wash-sale window", ticker)
				}
			}
		} else {
			utils.Verbosef(ctx, "Evaluating holdings outside the portfolio plan for tax losses")
		}

		harvestFunc := func(ticker string) models.SellProposal {
			holding, isHeld := collections.Find(slices.Values(holdingsByAsset[i]), func(holding portfolio.Holding) bool {
				return holding.Ticker == ticker
			})
			if !isHeld {
				return models.SellProposal{}
			}
			if snapshot.Portfolio.RecentTx.HasRecentBuy(ticker) {
				utils.Verbosef(ctx, "Skip harvesting %s: recent purchases in the wash-sale window", ticker)
				return models.SellProposal{}
			}

			candidate := rebalance.HarvestTaxLoss(holding, *settings.MinPctLossToHarvest)
			if len(candidate.Lots) == 0 {
				utils.Verbosef(ctx, "No lots of %s meet the %s%% loss threshold",
					ticker, settings.MinPctLossToHarvest.Shift(2).StringFixed(2))
			} else {
				utils.Verbosef(ctx, "Harvest candidate %s: %d lot(s), proceeds $%s, P/L $%s",
					ticker, len(candidate.Lots), candidate.Proceeds.StringFixed(2), candidate.RealizedPL.StringFixed(2))
			}
			return candidate
		}

		if asset.Plan == nil {
			// We should not force liquidation of unplanned positions while rebalancing,
			// but we should eagerly harvest any tax-loss these positions have accrued.
			for _, ticker := range asset.Tickers {

				if sell := harvestFunc(ticker); len(sell.Lots) > 0 {
					plan.Sells = append(plan.Sells, sell)
					balance = balance.Add(sell.Proceeds)
				}
			}
			continue
		}

		sell := rebalance.SelectPosToSell(asset, snapshot.Portfolio.RecentTx, harvestFunc)
		buy := rebalance.SelectPosToBuy(asset, snapshot.Portfolio.RecentTx, sell, quotePerTicker)

		if buy.Ticker == "" {
			// This should rarely happen, but if we cannot maintain an asset's allocation
			// by purchasing its paired ticker, we should not harvest a ticker
			if !sell.Proceeds.Sub(asset.ValueDrift).IsPositive() {
				utils.Verbosef(ctx, "Skip %s: no allocation shortfall after considering sales", asset.Plan.PrimaryTicker)
			} else {
				utils.Verbosef(ctx, "Skip %s: no eligible purchase ticker", asset.Plan.PrimaryTicker)
			}
			continue
		}

		utils.Verbosef(ctx, "Queue purchase of %s to cover a $%s shortfall", buy.Ticker, buy.ValueDrift.StringFixed(2))
		buyRequests = append(buyRequests, buy)

		if len(sell.Lots) > 0 {
			plan.Sells = append(plan.Sells, sell)
			balance = balance.Add(sell.Proceeds)
		}
	}

	log.Printf("Sizing %d purchase(s) with $%s available, including proposed sales", len(buyRequests), balance.StringFixed(2))
	for _, request := range buyRequests {
		if balance.Cmp(decimal.Zero) <= 0 {
			log.Print("No buying power remains; skipping remaining purchases")
			break
		}

		buy, err := rebalance.ProposeBuy(
			request,
			balance,
			maxValueDeviationAllowed,
			*settings.MinFractionalShareToRound,
			brokerSettings.AllowFractionalShares,
		)
		if err != nil {
			return nil, fmt.Errorf("size buy for %s: %w", request.Ticker, err)
		}

		if buy.Quantity.IsZero() {
			utils.Verbosef(ctx, "Skip buying %s: sized quantity is zero under cash and rounding limits", request.Ticker)
			continue
		}

		plan.Buys = append(plan.Buys, buy)
		balance = balance.Sub(buy.EstimatedCost)
		utils.Verbosef(ctx, "Propose buying %s: %s share(s), cost $%s, remaining balance $%s",
			buy.Ticker, buy.Quantity.String(), buy.EstimatedCost.StringFixed(2), balance.StringFixed(2))
	}

	log.Printf("Rebalance complete: %d buy(s), %d sell(s)", len(plan.Buys), len(plan.Sells))
	return &plan, nil
}
