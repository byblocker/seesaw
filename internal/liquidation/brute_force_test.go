package liquidation

import (
	"context"
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
	oracle, err := p.search(request)
	if err != nil {
		if !p.wholeShareValue().LessThan(plan.MinTargetSale) {
			t.Fatalf("seed %d: refused with %v, but holdings can raise %s of %s", seed, err, p.wholeShareValue(), plan.MinTargetSale)
		}
		return holdingsTooSmall
	}

	want, wantFound := bruteForce(oracle)

	search, _ := p.search(request)
	if err := search.run(context.Background()); err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	if !wantFound {
		if search.found {
			t.Fatalf("seed %d: search found %+v, but brute force found no plan in the funding window", seed, search.bestTotals)
		}
		return noWholeShareFit
	}
	if !search.found {
		t.Fatalf("seed %d: search found no plan, but brute force found %+v", seed, want)
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

	return planFound
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

// Recount the shortfall from scratch. The search keeps running totals instead; this stays as their reference.
func (search *saleSearch) shortfallBelowMin(sharesByPosition []int64) decimal.Decimal {
	shortfall := decimal.Zero
	for i := range search.assets {
		asset := &search.assets[i]
		sold := search.soldFrom(asset, sharesByPosition)
		shortfall = shortfall.Add(asset.shortfallBelowMin(sold))
	}

	return shortfall
}

func (search *saleSearch) soldFrom(asset *sellableAsset, sharesByPosition []int64) decimal.Decimal {
	sold := decimal.Zero
	for i, shares := range sharesByPosition {
		position := &search.positions[i]
		if position.asset == asset {
			sold = sold.Add(position.saleProceeds(shares))
		}
	}

	return sold
}
