package liquidation

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"seesaw/internal/utils"
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
	snapshot  portfolio.Snapshot
	quotes    map[string]models.Quote
	lots      map[string][]models.PositionLot
	nextLotID int64
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
		quotes: map[string]models.Quote{},
		lots:   map[string][]models.PositionLot{},
	}
}

// Add a planned asset, or an unplanned one when the target is empty. Add its holdings first.
func (p *testPortfolio) asset(target string, tickers ...string) *testPortfolio {
	asset := portfolio.Asset{Tickers: tickers}
	for _, ticker := range tickers {
		position := portfolio.Position{
			Ticker: ticker,
			Lots:   p.lots[ticker],
			Quote:  p.quotes[ticker],
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

func testRequest(amount, maxDeviation string, rates *config.TaxSettings) models.LiquidationRequest {
	return models.LiquidationRequest{
		Amount: decimal.RequireFromString(amount),
		Settings: config.LiquidationSettings{
			MaxAllocationDeviation: decimal.RequireFromString(maxDeviation),
		},
		TaxRates: rates,
	}
}

// What PlanSales should sell for: existing cash goes first, and sales cover the rest.
func (p *testPortfolio) amountNeeded(request models.LiquidationRequest) decimal.Decimal {
	return utils.ClampNonNegative(request.Amount.Sub(p.snapshot.Balances.CashBalance))
}

func (p *testPortfolio) planSales(t testing.TB, request models.LiquidationRequest) *models.LiquidationSelection {
	t.Helper()

	selection, err := PlanSales(p.snapshot, request)
	if err != nil {
		t.Fatal(err)
	}
	return selection
}

// Add up every whole share the portfolio could sell, without going through the planner,
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

// Build a small random portfolio. Report false when existing cash already covers the
// request, since there is nothing to sell.
func randomCase(seed int64) (*testPortfolio, models.LiquidationRequest, bool) {
	const maxAssets, maxTickers, maxLotShares = 3, 3, 12
	random := rand.New(rand.NewSource(seed))
	price := func(low, high float64) string {
		return decimal.NewFromFloat(low + random.Float64()*(high-low)).StringFixed(2)
	}

	cash := price(0, 20)
	p := newTestPortfolio(cash)

	assetCount := 1 + random.Intn(maxAssets)
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
			if tickers == maxTickers {
				break
			}
			ticker := fmt.Sprintf("T%d", tickers)
			tickers++
			assetTickers = append(assetTickers, ticker)

			bid := price(5, 65)
			var lots []testLot
			for range 1 + random.Intn(2) {
				shares := decimal.NewFromInt(int64(1 + random.Intn(maxLotShares)))
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
	var rates *config.TaxSettings
	if random.Intn(4) != 0 {
		rates = testTaxRates
	}
	amount := price(1, holdingsValue*0.7+1)
	if random.Intn(20) == 0 {
		amount = price(holdingsValue, holdingsValue*1.3+1)
	}
	request := testRequest(amount, deviations[random.Intn(len(deviations))], rates)

	needsSale := p.amountNeeded(request).IsPositive()
	return p, request, needsSale
}
