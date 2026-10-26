package liquidation

import (
	"strings"
	"testing"

	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func TestUnderweightAssetIsNotSold(t *testing.T) {
	p := newTestPortfolio("0").
		holding("VOO", "500", testLot{"100", "400", 24}, testLot{"80", "520", 24}).
		holding("STIP", "100", testLot{"10", "99", 24}).
		holding("VTIP", "50", testLot{"10", "49", 24}).
		asset("0.75", "VOO").
		asset("0.25", "STIP", "VTIP")

	selection := p.planSales(t, testRequest("7000", "0.10", testTaxRates))

	for _, sell := range selection.Sells {
		if sell.Ticker != "VOO" {
			t.Errorf("sold %s from the underweight asset", sell.Ticker)
		}
	}
	if proceeds := soldProceeds(selection); !proceeds.Equal(decimal.NewFromInt(7000)) {
		t.Errorf("proceeds %s, want 7000", proceeds)
	}
}

func TestSellsLossesBeforeGains(t *testing.T) {
	p := newTestPortfolio("0").
		holding("GAIN", "100", testLot{"10", "50", 24}).
		holding("LOSS", "100", testLot{"10", "150", 24}).
		asset("0.5", "GAIN").
		asset("0.5", "LOSS")

	selection := p.planSales(t, testRequest("500", "0.5", testTaxRates))

	if len(selection.Sells) != 1 || selection.Sells[0].Ticker != "LOSS" {
		t.Errorf("sold %+v, want only LOSS", selection.Sells)
	}
}

func TestDoesNotSellExtraLosses(t *testing.T) {
	p := newTestPortfolio("0").
		holding("LOSS", "100", testLot{"50", "150", 24}).
		asset("1", "LOSS")

	selection := p.planSales(t, testRequest("1000", "0", testTaxRates))

	if proceeds := soldProceeds(selection); !proceeds.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("proceeds %s, want exactly the 1000 requested", proceeds)
	}
}

func TestSellsUnplannedHoldingsFirst(t *testing.T) {
	p := newTestPortfolio("0").
		holding("PLANNED", "100", testLot{"10", "150", 24}).
		holding("UNPLANNED", "100", testLot{"2", "50", 24}).
		asset("1", "PLANNED").
		asset("", "UNPLANNED")

	selection := p.planSales(t, testRequest("300", "1", testTaxRates))

	var got []string
	for _, sell := range selection.Sells {
		for _, sale := range sell.Lots {
			got = append(got, sell.Ticker+" x"+sale.Quantity.String())
		}
	}
	if strings.Join(got, ", ") != "UNPLANNED x2, PLANNED x1" {
		t.Errorf("sold %v, want the unplanned gain before the planned loss", got)
	}
}

func TestReportsImpossibleRequests(t *testing.T) {
	for _, test := range []struct {
		name      string
		portfolio *testPortfolio
		amount    string
		want      string
	}{
		{
			name: "holdings too small",
			portfolio: newTestPortfolio("0").
				holding("ONLY", "30", testLot{"10", "30", 24}).
				asset("1", "ONLY"),
			amount: "1000",
			want:   "whole-share holdings can raise at most $300.00 of the $1000.00 needed",
		},
		{
			// OTHER's minimum is zero, but half its value is a fractional share. MAIN is at its minimum.
			name: "minimums block the sale",
			portfolio: newTestPortfolio("0").
				holding("OTHER", "100", testLot{"1.5", "100", 24}).
				holding("MAIN", "100", testLot{"10", "100", 24}).
				asset("0", "OTHER").
				asset("1", "MAIN"),
			amount: "140",
			want:   "sales within allocation minimums raise at most $100.00 of the $140.00 needed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := test.portfolio
			_, err := PlanSales(p.snapshot, testRequest(test.amount, "0", nil))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %v, want %q", err, test.want)
			}
		})
	}
}

func TestPlanSalesOnRandomPortfolios(t *testing.T) {
	seeds := int64(3000)
	if testing.Short() {
		seeds /= 10
	}

	plans, holdingsTooSmall, minimumsBlocked := 0, 0, 0
	for seed := range seeds {
		p, request, needsSale := randomCase(seed)
		if !needsSale {
			continue
		}

		amountNeeded := p.amountNeeded(request)
		selection, err := PlanSales(p.snapshot, request)
		switch {
		case p.wholeShareValue().LessThan(amountNeeded):
			if err == nil || !strings.Contains(err.Error(), "whole-share holdings can raise at most") {
				t.Fatalf("seed %d: error %v, want holdings too small", seed, err)
			}
			holdingsTooSmall++
		case err != nil:
			if !strings.Contains(err.Error(), "within allocation minimums") {
				t.Fatalf("seed %d: %v", seed, err)
			}
			minimumsBlocked++
		default:
			highestBid := decimal.Zero
			for _, quote := range p.quotes {
				highestBid = decimal.Max(highestBid, quote.Bid)
			}
			checkSelection(t, seed, p, request, amountNeeded.Add(highestBid), selection)
			plans++
		}
	}

	// Each outcome must actually occur, or a fixture change has quietly stopped testing it.
	if plans < int(seeds)/2 || holdingsTooSmall == 0 {
		t.Fatalf("plans %d of %d seeds, holdings too small %d", plans, seeds, holdingsTooSmall)
	}
	t.Logf("plans %d, holdings too small %d, minimums blocked %d", plans, holdingsTooSmall, minimumsBlocked)
}

// Check what users see: real orders that sell whole shares from lots they hold, raise the
// amount needed, and stay within maxSale.
func checkSelection(
	t *testing.T,
	seed int64,
	p *testPortfolio,
	request models.LiquidationRequest,
	maxSale decimal.Decimal,
	selection *models.LiquidationSelection,
) {
	t.Helper()

	proceeds := decimal.Zero
	for _, proposal := range selection.Sells {
		if !proposal.Price.IsPositive() {
			t.Fatalf("seed %d: sells %s at %s", seed, proposal.Ticker, proposal.Price)
		}
		for _, sale := range proposal.Lots {
			if !sale.Quantity.IsPositive() || !sale.Quantity.Equal(sale.Quantity.Floor()) || sale.Quantity.GreaterThan(sale.Lot.Quantity.Floor()) {
				t.Fatalf("seed %d: sells %s of %s from a lot of %s", seed, sale.Quantity, sale.Ticker, sale.Lot.Quantity)
			}
			proceeds = proceeds.Add(sale.Proceeds())
		}
	}

	amountNeeded := p.amountNeeded(request)
	if proceeds.LessThan(amountNeeded) || proceeds.GreaterThan(maxSale) {
		t.Fatalf("seed %d: proceeds %s outside %s to %s", seed, proceeds, amountNeeded, maxSale)
	}
}

func soldProceeds(selection *models.LiquidationSelection) decimal.Decimal {
	proceeds := decimal.Zero
	for _, sell := range selection.Sells {
		proceeds = proceeds.Add(sell.Proceeds)
	}
	return proceeds
}
