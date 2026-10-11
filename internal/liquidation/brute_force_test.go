package liquidation

import (
	"context"
	"strings"
	"testing"

	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

func TestSearchMatchesBruteForce(t *testing.T) {
	outcomes := map[bruteForceOutcome]int{}
	for _, test := range []struct {
		name  string
		shape randomShape
		seeds int64
	}{
		{"small", smallShape, 5000},
		{"wide", wideShape, 300},
	} {
		t.Run(test.name, func(t *testing.T) {
			seeds := test.seeds
			if testing.Short() {
				seeds /= 10
			}

			for seed := range seeds {
				if p, request, needsSale := randomCase(seed, test.shape); needsSale {
					outcomes[checkAgainstBruteForce(t, seed, p, request)]++
				}
			}
		})
	}

	// Each outcome must actually occur, or a fixture change has quietly stopped testing it.
	minPlans := 2000
	if testing.Short() {
		minPlans /= 10
	}
	if outcomes[planFound] < minPlans || outcomes[holdingsTooSmall] == 0 || outcomes[noWholeShareFit] == 0 {
		t.Fatalf("plans %d (want at least %d), holdings too small %d, no whole-share fit %d",
			outcomes[planFound], minPlans, outcomes[holdingsTooSmall], outcomes[noWholeShareFit])
	}
	t.Logf("plans %d, holdings too small %d, no whole-share fit %d",
		outcomes[planFound], outcomes[holdingsTooSmall], outcomes[noWholeShareFit])
}

// Search seeds the fixed tests do not reach. Run with: go test -fuzz=FuzzSearchMatchesBruteForce
func FuzzSearchMatchesBruteForce(f *testing.F) {
	f.Add(int64(0), false)
	f.Add(int64(1), true)
	f.Fuzz(func(t *testing.T, seed int64, wide bool) {
		shape := smallShape
		if wide {
			shape = wideShape
		}
		if p, request, needsSale := randomCase(seed, shape); needsSale {
			checkAgainstBruteForce(t, seed, p, request)
		}
	})
}

type bruteForceOutcome int

const (
	planFound bruteForceOutcome = iota
	holdingsTooSmall
	noWholeShareFit
)

func checkAgainstBruteForce(t *testing.T, seed int64, p *testPortfolio, request models.LiquidationRequest) bruteForceOutcome {
	t.Helper()

	plan := p.plan(request)
	selection, err := PlanSales(context.Background(), p.snapshot, p.quotes, testSaleDate, plan)

	oracle, oracleErr := p.search(request)
	if oracleErr != nil {
		if !p.wholeShareValue().LessThan(plan.MinTargetSale) {
			t.Fatalf("seed %d: refused with %v, but holdings can raise %s of %s", seed, oracleErr, p.wholeShareValue(), plan.MinTargetSale)
		}
		if err == nil || !strings.Contains(err.Error(), "can raise at most") {
			t.Fatalf("seed %d: error %v, want holdings too small", seed, err)
		}
		return holdingsTooSmall
	}

	want, wantFound := bruteForce(oracle)
	if !wantFound {
		if err == nil || !strings.Contains(err.Error(), "no whole-share sale raises") {
			t.Fatalf("seed %d: error %v, but brute force found no plan in the funding window", seed, err)
		}
		return noWholeShareFit
	}
	if err != nil {
		t.Fatalf("seed %d: %v, but brute force found %+v", seed, err, want)
	}

	search, _ := p.search(request)
	if err := search.run(context.Background()); err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}

	got := search.bestTotals
	tolerance := request.Amount.Mul(pctProceedsTolerance)
	switch {
	case !got.shortfallBelowMin.Equal(want.shortfallBelowMin):
		t.Fatalf("seed %d: shortfall %s, brute force %s", seed, got.shortfallBelowMin, want.shortfallBelowMin)
	case !got.taxOwed().Equal(want.taxOwed()):
		t.Fatalf("seed %d: tax owed %s, brute force %s", seed, got.taxOwed(), want.taxOwed())
	case got.proceeds.LessThan(want.proceeds) || got.proceeds.Sub(want.proceeds).GreaterThan(tolerance):
		t.Fatalf("seed %d: proceeds %s, brute force %s within %s", seed, got.proceeds, want.proceeds, tolerance)
	}

	checkSelection(t, seed, plan, oracle.maxSale, selection)
	return planFound
}

// Check what users see: real orders that sell whole shares from lots they hold, with
// proceeds and balance that add up and stay inside the funding window.
func checkSelection(t *testing.T, seed int64, plan *models.LiquidationPlan, maxSale decimal.Decimal, selection *models.LiquidationSelection) {
	t.Helper()

	proceeds := decimal.Zero
	for _, proposal := range selection.Sells {
		if err := proposal.Validate(); err != nil {
			t.Fatalf("seed %d: %s: %v", seed, proposal.Ticker, err)
		}
		for _, sale := range proposal.Lots {
			if !sale.Quantity.Equal(sale.Quantity.Floor()) || sale.Quantity.GreaterThan(sale.Lot.Quantity.Floor()) {
				t.Fatalf("seed %d: sells %s of %s from a lot of %s", seed, sale.Quantity, sale.Ticker, sale.Lot.Quantity)
			}
			proceeds = proceeds.Add(sale.Proceeds())
		}
	}

	expectedBalance := plan.StartingCash.Sub(plan.Request.Amount).Add(selection.Proceeds)
	switch {
	case !proceeds.Equal(selection.Proceeds):
		t.Fatalf("seed %d: lot sales raise %s, selection reports %s", seed, proceeds, selection.Proceeds)
	case proceeds.LessThan(plan.MinTargetSale) || proceeds.GreaterThan(maxSale):
		t.Fatalf("seed %d: proceeds %s outside %s to %s", seed, proceeds, plan.MinTargetSale, maxSale)
	case !selection.ExpectedBalance.Equal(expectedBalance):
		t.Fatalf("seed %d: expected balance %s, want %s", seed, selection.ExpectedBalance, expectedBalance)
	}
}

// Score every combination of share counts with the same totals the search uses, as the ground truth.
func bruteForce(search *saleSearch) (best saleTotals, found bool) {
	var visit func(depth int)
	visit = func(depth int) {
		if depth < len(search.positions) {
			for shares := search.positions[depth].sellableShares; shares >= 0; shares-- {
				search.chosenSharesByPosition[depth] = shares
				visit(depth + 1)
			}
			return
		}

		var totals saleTotals
		for i, shares := range search.chosenSharesByPosition {
			position := &search.positions[i]
			totals = totals.withSale(position.saleProceeds(shares), position.estimateSaleTax(shares))
		}
		if totals.proceeds.LessThan(search.minSale) || totals.proceeds.GreaterThan(search.maxSale) {
			return
		}

		totals.shortfallBelowMin = search.shortfallBelowMin(search.chosenSharesByPosition)
		if !found || totals.compare(best) > 0 {
			best, found = totals, true
		}
	}

	visit(0)
	return best, found
}
