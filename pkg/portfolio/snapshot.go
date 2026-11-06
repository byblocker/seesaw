package portfolio

import (
	"cmp"
	"context"
	"fmt"
	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const DivisionDecimalPlaces int32 = 19

type Snapshot struct {
	AccountId  string
	CreatedAt  time.Time
	Balances   models.Balances
	Assets     []Asset
	Positions  []Position
	Lots       []Lot
	TotalValue decimal.Decimal

	TaxRates *config.TaxSettings // Nil for non-taxable accounts
	Broker   models.BrokerSettings
}

type Asset struct {
	Plan         *config.AssetPlan
	Label        string
	PositionIdxs []int

	Allocation decimal.Decimal
	AllocDrift decimal.Decimal

	Value      decimal.Decimal
	ValueDrift decimal.Decimal
}

type Position struct {
	Ticker         string
	AssetIdx       int
	LotIdxs        []int
	Quote          models.Quote
	RecentlyBought bool
	RecentlySold   bool
}

type Lot struct {
	models.TaxLot
	AssetIdx    int
	PositionIdx int
	Snapshot    LotSnapshot
}

// What selling the lot at the snapshot's bid on its date would realize, and how it would be taxed.
type LotSnapshot struct {
	Bid          decimal.Decimal
	GainPerShare decimal.Decimal
	GainPct      decimal.Decimal // Fraction of the price paid
	LongTerm     bool
	IsWashSale   bool            // A taxable loss that a recent purchase disallows
	TaxPerShare  decimal.Decimal // Zero for non-taxable accounts and wash sales
	// Truncating the quotient never reorders lots, so sorting by it matches the exact ratio up to ties below its precision.
	TaxPerDollar decimal.Decimal
}

func TakeSnapshot(
	ctx context.Context,
	client models.BrokerClient,
	plan *config.Plan,
	taxRates *config.TaxSettings,
) (Snapshot, error) {
	if err := taxRates.Validate(); err != nil {
		return Snapshot{}, err
	}
	portfolio, err := client.Portfolio(
		ctx,
		plan.AccountId,
		transactionHistoryDays,
		plan.Tickers(),
	)
	if err != nil {
		return Snapshot{}, fmt.Errorf("failed to retrieve portfolio: %w", err)
	}

	createdAt := time.Now()
	assets, positions, positionIdxByTicker := newAssets(plan.Assets, portfolio.Lots)
	if err := setQuotes(positions, positionIdxByTicker, portfolio.Quotes); err != nil {
		return Snapshot{}, err
	}
	flagRecentTrades(positions, positionIdxByTicker, portfolio.Transactions, createdAt)
	lots := newLots(positions, positionIdxByTicker, portfolio.Lots, createdAt, taxRates)
	addPositionValues(assets, positions, lots)
	totalValue := allocate(assets, portfolio.Balances.CashBalance)

	return Snapshot{
		AccountId:  plan.AccountId,
		CreatedAt:  createdAt,
		Balances:   portfolio.Balances,
		Assets:     assets,
		Positions:  positions,
		Lots:       lots,
		TotalValue: totalValue,
		TaxRates:   taxRates,
		Broker:     client.Settings(),
	}, nil
}

// One asset row per plan, then one for holdings outside the plan, each with a position row per ticker.
func newAssets(plans []config.AssetPlan, taxLots []models.TaxLot) ([]Asset, []Position, map[string]int) {
	assets := make([]Asset, 0, len(plans)+1) // includes asset for unplanned positions
	var positions []Position
	positionIdxByTicker := map[string]int{}
	addAsset := func(plan *config.AssetPlan, label string, tickers []string) {
		asset := Asset{Plan: plan, Label: label}
		for _, ticker := range tickers {
			positionIdxByTicker[ticker] = len(positions)
			asset.PositionIdxs = append(asset.PositionIdxs, len(positions))
			positions = append(positions, Position{Ticker: ticker, AssetIdx: len(assets)})
		}
		assets = append(assets, asset)
	}

	for i := range plans {
		tickers := plans[i].Tickers()
		addAsset(&plans[i], strings.Join(tickers, " / "), tickers)
	}

	var unplanned []string
	for _, lot := range taxLots {
		if _, planned := positionIdxByTicker[lot.Ticker]; !planned {
			unplanned = append(unplanned, lot.Ticker)
		}
	}
	if len(unplanned) > 0 {
		// Brokers list lots in no set order; sort so ties between unplanned positions break the same way every run.
		slices.Sort(unplanned)
		addAsset(nil, "OTHER", slices.Compact(unplanned))
	}

	return assets, positions, positionIdxByTicker
}

// Write each quote into its position, and reject any position left without a positive bid and ask.
func setQuotes(positions []Position, positionIdxByTicker map[string]int, quotes []models.Quote) error {
	for _, quote := range quotes {
		if positionIdx, ok := positionIdxByTicker[quote.Ticker]; ok {
			positions[positionIdx].Quote = quote
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
		positionIdx, ok := positionIdxByTicker[tx.Ticker]
		if !ok || daysBetween(tx.Date, createdAt) > WashSaleWindowDays {
			continue
		}
		switch tx.Type {
		case models.TxBuy:
			positions[positionIdx].RecentlyBought = true
		case models.TxSell:
			positions[positionIdx].RecentlySold = true
		}
	}
}

// A lot row for every held lot, recorded in its position's LotIdxs in lot ID order.
func newLots(
	positions []Position,
	positionIdxByTicker map[string]int,
	taxLots []models.TaxLot,
	createdAt time.Time,
	taxRates *config.TaxSettings,
) []Lot {
	var lots []Lot
	for _, lot := range taxLots {
		if !lot.Quantity.IsPositive() {
			continue
		}
		positionIdx := positionIdxByTicker[lot.Ticker] // newAssets gave every held ticker a position
		position := &positions[positionIdx]
		position.LotIdxs = append(position.LotIdxs, len(lots))
		lots = append(lots, Lot{
			TaxLot:      lot,
			AssetIdx:    position.AssetIdx,
			PositionIdx: positionIdx,
			Snapshot:    newLotSnapshot(lot, *position, createdAt, taxRates),
		})
	}

	// Brokers list lots in no set order; sort by lot ID so plans repeat exactly.
	for positionIdx := range positions {
		slices.SortFunc(positions[positionIdx].LotIdxs, func(leftLotIdx, rightLotIdx int) int {
			return cmp.Compare(lots[leftLotIdx].Id, lots[rightLotIdx].Id)
		})
	}
	return lots
}

func newLotSnapshot(lot models.TaxLot, position Position, createdAt time.Time, taxRates *config.TaxSettings) LotSnapshot {
	bid := position.Quote.Bid
	gainPerShare := bid.Sub(lot.PricePaid)
	gainPct := decimal.Zero
	if lot.PricePaid.IsPositive() {
		gainPct, _ = gainPerShare.QuoRem(lot.PricePaid, DivisionDecimalPlaces)
	}

	snapshot := LotSnapshot{
		Bid:          bid,
		GainPerShare: gainPerShare,
		GainPct:      gainPct,
		LongTerm:     isLongTerm(lot.TimeAcquired, createdAt),
		IsWashSale:   taxRates != nil && position.RecentlyBought && gainPerShare.IsNegative(),
	}
	snapshot.TaxPerShare = snapshot.taxPerShare(taxRates)
	snapshot.TaxPerDollar, _ = snapshot.TaxPerShare.QuoRem(bid, DivisionDecimalPlaces)
	return snapshot
}

// Add each position's holdings at its bid to its asset's value.
func addPositionValues(assets []Asset, positions []Position, lots []Lot) {
	for _, position := range positions {
		quantity := decimal.Zero
		for _, lotIdx := range position.LotIdxs {
			quantity = quantity.Add(lots[lotIdx].Quantity)
		}
		asset := &assets[position.AssetIdx]
		asset.Value = asset.Value.Add(quantity.Mul(position.Quote.Bid))
	}
}

// Set each asset's allocation and drift from the asset values plus cash, and return that total.
func allocate(assets []Asset, cash decimal.Decimal) decimal.Decimal {
	totalValue := cash
	for _, asset := range assets {
		totalValue = totalValue.Add(asset.Value)
	}

	for assetIdx := range assets {
		asset := &assets[assetIdx]
		asset.Allocation = decimal.Zero
		if !totalValue.IsZero() {
			asset.Allocation, _ = asset.Value.QuoRem(totalValue, DivisionDecimalPlaces)
		}

		if asset.Plan != nil {
			asset.AllocDrift = asset.Allocation.Sub(asset.Plan.TargetAllocation)
			expectedValue := totalValue.Mul(asset.Plan.TargetAllocation)
			asset.ValueDrift = asset.Value.Sub(expectedValue)
		}
	}
	return totalValue
}

// The snapshot after the proposal. Its positions and lots are the snapshot's own, unchanged.
func (snapshot Snapshot) Simulate(proposal Proposal) Snapshot {
	simulated := snapshot
	simulated.Assets = slices.Clone(snapshot.Assets)
	cash := snapshot.Balances.CashBalance.Sub(proposal.Withdrawal)
	for _, sell := range proposal.Sells {
		asset := &simulated.Assets[snapshot.Positions[sell.PositionIdx].AssetIdx]
		asset.Value = asset.Value.Sub(sell.Proceeds)
		cash = cash.Add(sell.Proceeds)
	}
	for _, buy := range proposal.Buys {
		asset := &simulated.Assets[snapshot.Positions[buy.PositionIdx].AssetIdx]
		asset.Value = asset.Value.Add(buy.EstimatedCost)
		cash = cash.Sub(buy.EstimatedCost)
	}

	simulated.Balances.CashBalance = cash
	simulated.TotalValue = allocate(simulated.Assets, cash)
	return simulated
}
