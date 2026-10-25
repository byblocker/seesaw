package liquidation

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"seesaw/pkg/config"
	"seesaw/pkg/models"
	"seesaw/pkg/portfolio"

	"github.com/shopspring/decimal"
)

var testSaleDate = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

var testTaxRates = &config.TaxSettings{
	ShortTermGainRate: decimal.RequireFromString("0.35"),
	LongTermGainRate:  decimal.RequireFromString("0.20"),
}

type testPortfolio struct {
	snapshot       portfolio.Snapshot
	quotes         map[string]models.Quote
	lots           map[string][]models.PositionLot
	recentlyBought map[string]bool
	nextLotID      int64
}

type testLot struct {
	shares     string
	paid       string
	monthsHeld int
}

func newTestPortfolio(cash string) *testPortfolio {
	balance := decimal.RequireFromString(cash)
	return &testPortfolio{
		snapshot: portfolio.Snapshot{
			CreatedAt: testSaleDate,
			Balances:  models.Balances{CashBalance: balance},
		},
		quotes:         map[string]models.Quote{},
		lots:           map[string][]models.PositionLot{},
		recentlyBought: map[string]bool{},
	}
}

// Add a planned asset, or an unplanned one when the target is empty. Add its holdings first.
func (p *testPortfolio) asset(target string, tickers ...string) *testPortfolio {
	asset := portfolio.Asset{Tickers: tickers}
	for _, ticker := range tickers {
		position := portfolio.Position{
			Ticker:         ticker,
			Lots:           p.lots[ticker],
			Quote:          p.quotes[ticker],
			RecentlyBought: p.recentlyBought[ticker],
		}
		asset.Positions = append(asset.Positions, position)
		asset.Value = asset.Value.Add(position.Value())
	}
	if target != "" {
		asset.Plan = &config.AssetPlan{TargetAllocation: decimal.RequireFromString(target)}
	}
	p.snapshot.Assets = append(p.snapshot.Assets, asset)
	return p
}

// Set the drift of the asset added last.
func (p *testPortfolio) drift(drift string) *testPortfolio {
	p.snapshot.Assets[len(p.snapshot.Assets)-1].AllocDrift = decimal.RequireFromString(drift)
	return p
}

// Mark a ticker as bought within the wash-sale window. Call before adding its asset.
func (p *testPortfolio) boughtRecently(ticker string) *testPortfolio {
	p.recentlyBought[ticker] = true
	return p
}

func (p *testPortfolio) holding(ticker, bid string, lots ...testLot) *testPortfolio {
	for _, lot := range lots {
		p.nextLotID++
		p.lots[ticker] = append(p.lots[ticker], models.PositionLot{
			Id:           p.nextLotID,
			Quantity:     decimal.RequireFromString(lot.shares),
			PricePaid:    decimal.RequireFromString(lot.paid),
			TimeAcquired: testSaleDate.AddDate(0, -lot.monthsHeld, 0),
		})
	}

	price := decimal.RequireFromString(bid)
	p.quotes[ticker] = models.Quote{Bid: price, Ask: price}
	return p
}

func testRequest(amount, maxDeviation, maxExcessCash string, rates *config.TaxSettings) models.LiquidationRequest {
	return models.LiquidationRequest{
		Amount: decimal.RequireFromString(amount),
		Settings: config.LiquidationSettings{
			MaxAllocationDeviation: decimal.RequireFromString(maxDeviation),
			MaxExcessCash:          decimal.RequireFromString(maxExcessCash),
		},
		TaxRates:    rates,
		SearchLimit: 10_000_000,
	}
}

func (p *testPortfolio) plan(request models.LiquidationRequest) *models.LiquidationPlan {
	return PlanFunding(p.snapshot.Balances, request)
}

func (p *testPortfolio) planSales(t testing.TB, request models.LiquidationRequest) *models.LiquidationSelection {
	t.Helper()

	selection, err := PlanSales(context.Background(), p.snapshot, p.plan(request))
	if err != nil {
		t.Fatal(err)
	}
	return selection
}

// Add up every whole share the portfolio could sell, without going through the search,
// so tests can tell when a request is impossible.
func (p *testPortfolio) wholeShareValue() decimal.Decimal {
	value := decimal.Zero
	for _, asset := range p.snapshot.Assets {
		for _, ticker := range asset.Tickers {
			for _, lot := range p.lots[ticker] {
				value = value.Add(lot.Quantity.Floor().Mul(p.quotes[ticker].Bid))
			}
		}
	}
	return value
}

func (p *testPortfolio) search(request models.LiquidationRequest) (*saleSearch, error) {
	plan := p.plan(request)
	positions, assets := collectPositions(p.snapshot, plan)
	return newSaleSearch(positions, assets, plan)
}

type randomShape struct {
	maxAssets    int
	maxTickers   int
	maxLotShares int
}

var (
	smallShape = randomShape{maxAssets: 3, maxTickers: 4, maxLotShares: 4}
	wideShape  = randomShape{maxAssets: 3, maxTickers: 3, maxLotShares: 12}
)

// Build a portfolio small enough for brute force to try every share count. Report
// false when existing cash already covers the request, since there is nothing to search.
func randomCase(seed int64, shape randomShape) (*testPortfolio, models.LiquidationRequest, bool) {
	random := rand.New(rand.NewSource(seed))
	price := func(low, high float64) string {
		return decimal.NewFromFloat(low + random.Float64()*(high-low)).StringFixed(2)
	}

	cash := price(0, 20)
	p := newTestPortfolio(cash)

	assetCount := 1 + random.Intn(shape.maxAssets)
	hasUnplanned := assetCount > 1 && random.Intn(3) == 0
	weights := make([]float64, assetCount)
	total := 0.0
	for i := range weights {
		weights[i] = 0.1 + random.Float64()
		total += weights[i]
	}

	tickers, holdingsValue := 0, 0.0
	for i := range assetCount {
		var assetTickers []string
		for range 1 + random.Intn(2) {
			if tickers == shape.maxTickers {
				break
			}
			ticker := fmt.Sprintf("T%d", tickers)
			tickers++
			assetTickers = append(assetTickers, ticker)

			bid := price(5, 65)
			var lots []testLot
			for range 1 + random.Intn(2) {
				shares := decimal.NewFromInt(int64(1 + random.Intn(shape.maxLotShares)))
				if random.Intn(4) == 0 {
					shares = shares.Add(decimal.NewFromFloat(random.Float64()).Round(2))
				}
				paid := decimal.RequireFromString(bid).Mul(decimal.NewFromFloat(0.5 + random.Float64())).StringFixed(2)
				lots = append(lots, testLot{shares: shares.String(), paid: paid, monthsHeld: random.Intn(30)})
				holdingsValue += shares.InexactFloat64() * decimal.RequireFromString(bid).InexactFloat64()
			}
			p.holding(ticker, bid, lots...)
		}

		target := decimal.NewFromFloat(weights[i] / total).StringFixed(2)
		if hasUnplanned && i == assetCount-1 {
			target = ""
		}
		p.asset(target, assetTickers...)
	}

	deviations := []string{"0", "0.02", "0.1", "0.3"}
	excesses := []string{"0", "3", "15", "60"}
	var rates *config.TaxSettings
	if random.Intn(4) != 0 {
		rates = testTaxRates
	}
	amount := price(1, holdingsValue*0.7+1)
	if random.Intn(20) == 0 {
		amount = price(holdingsValue, holdingsValue*1.3+1)
	}
	request := testRequest(
		amount,
		deviations[random.Intn(len(deviations))],
		excesses[random.Intn(len(excesses))],
		rates,
	)

	needsSale := p.plan(request).MinTargetSale.IsPositive()
	return p, request, needsSale
}

// Build a portfolio sized like a real account, with three assets of two tickers each and
// hundreds of shares split across many lots, where share-count ranges are wide.
func realisticCase(seed int64, amount, maxExcessCash, maxDeviation string) (*testPortfolio, models.LiquidationRequest) {
	random := rand.New(rand.NewSource(seed))
	type ticker struct {
		name      string
		bid       float64
		minShares int
		maxShares int
	}
	assets := []struct {
		target  string
		tickers []ticker
	}{
		{"0.60", []ticker{{"VOO", 500 + random.Float64()*50, 60, 140}, {"SCHX", 24 + random.Float64()*3, 300, 900}}},
		{"0.25", []ticker{{"STIP", 100 + random.Float64()*3, 50, 200}, {"VTIP", 49 + random.Float64()*2, 50, 250}}},
		{"0.15", []ticker{{"SGOV", 100 + random.Float64(), 50, 200}, {"BIL", 91 + random.Float64(), 20, 120}}},
	}

	p := newTestPortfolio("0")
	for _, asset := range assets {
		var names []string
		for _, t := range asset.tickers {
			names = append(names, t.name)
			var lots []testLot
			for remaining := t.minShares + random.Intn(t.maxShares-t.minShares); remaining > 0; {
				shares := min(remaining, 5+random.Intn(40))
				remaining -= shares
				paid := decimal.NewFromFloat(t.bid * (0.7 + random.Float64()*0.5)).StringFixed(2)
				lots = append(lots, testLot{shares: fmt.Sprint(shares), paid: paid, monthsHeld: random.Intn(36)})
			}
			p.holding(t.name, decimal.NewFromFloat(t.bid).StringFixed(2), lots...)
		}
		p.asset(asset.target, names...)
	}

	return p, testRequest(amount, maxDeviation, maxExcessCash, testTaxRates)
}
