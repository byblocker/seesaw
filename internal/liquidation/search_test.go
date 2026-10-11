package liquidation

import (
	"context"
	"math/rand"
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

func TestUnderweightAssetIsNotSold(t *testing.T) {
	p := newTestPortfolio("0").
		holding("VOO", "500", testLot{"100", "400", 24}, testLot{"80", "520", 24}).
		holding("STIP", "100", testLot{"10", "99", 24}).
		holding("VTIP", "50", testLot{"10", "49", 24}).
		asset("0.75", "VOO").
		asset("0.25", "STIP", "VTIP")

	selection := p.planSales(t, testRequest("7000", "0.10", "500", testTaxRates))

	for _, sell := range selection.Sells {
		if sell.Ticker != "VOO" {
			t.Errorf("sold %s from the underweight asset", sell.Ticker)
		}
	}
	if !selection.Proceeds.Equal(decimal.NewFromInt(7000)) {
		t.Errorf("proceeds %s, want 7000", selection.Proceeds)
	}

	var warned []string
	for _, warning := range selection.AllocationWarnings {
		warned = append(warned, warning.Asset)
	}
	if strings.Join(warned, ", ") != "VOO, STIP / VTIP" {
		t.Errorf("allocation warnings for %v, want VOO and STIP / VTIP", warned)
	}
}

func TestNoWarningsWhenAllocationsStayInBand(t *testing.T) {
	p := newTestPortfolio("0").
		holding("STOCK", "100", testLot{"75", "90", 24}).
		holding("BOND", "100", testLot{"25", "90", 24}).
		asset("0.75", "STOCK").
		asset("0.25", "BOND")

	selection := p.planSales(t, testRequest("1000", "0.05", "0", testTaxRates))

	if len(selection.AllocationWarnings) != 0 {
		t.Errorf("allocation warnings %+v, want none", selection.AllocationWarnings)
	}
}

func TestSellsLossesBeforeGains(t *testing.T) {
	p := newTestPortfolio("0").
		holding("GAIN", "100", testLot{"10", "50", 24}).
		holding("LOSS", "100", testLot{"10", "150", 24}).
		asset("0.5", "GAIN").
		asset("0.5", "LOSS")

	selection := p.planSales(t, testRequest("500", "0.5", "0", testTaxRates))

	if len(selection.Sells) != 1 || selection.Sells[0].Ticker != "LOSS" {
		t.Errorf("sold %+v, want only LOSS", selection.Sells)
	}
}

func TestDoesNotSellExtraLosses(t *testing.T) {
	p := newTestPortfolio("0").
		holding("LOSS", "100", testLot{"50", "150", 24}).
		asset("1", "LOSS")

	selection := p.planSales(t, testRequest("1000", "0", "500", testTaxRates))

	if !selection.Proceeds.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("proceeds %s, want exactly the 1000 requested", selection.Proceeds)
	}
}

func TestSellsUnplannedHoldingsOnTies(t *testing.T) {
	p := newTestPortfolio("0").
		holding("PLANNED", "100", testLot{"10", "100", 24}).
		holding("UNPLANNED", "100", testLot{"10", "100", 24}).
		asset("1", "PLANNED").
		asset("", "UNPLANNED")

	selection := p.planSales(t, testRequest("300", "1", "0", nil))

	if len(selection.Sells) != 1 || selection.Sells[0].Ticker != "UNPLANNED" {
		t.Errorf("sold %+v, want only UNPLANNED", selection.Sells)
	}
}

func TestKeepsCashReserve(t *testing.T) {
	p := newTestPortfolio("500").
		holding("ONLY", "50", testLot{"20", "50", 24}).
		asset("1", "ONLY")
	request := testRequest("400", "1", "0", nil)
	request.Settings.CashReserve = decimalPointer("300")

	selection := p.planSales(t, request)

	if !selection.Proceeds.Equal(decimal.NewFromInt(200)) {
		t.Errorf("proceeds %s, want the 200 that keeps the 300 reserve", selection.Proceeds)
	}
	if !selection.ExpectedBalance.Equal(decimal.NewFromInt(300)) {
		t.Errorf("expected balance %s, want the 300 reserve", selection.ExpectedBalance)
	}
}

func TestReportsImpossibleRequests(t *testing.T) {
	for _, test := range []struct {
		name      string
		amount    string
		maxExcess string
		want      string
	}{
		{"holdings too small", "1000", "0", "can raise at most $300.00"},
		{"no whole-share fit", "100", "0", "no whole-share sale raises between $100.00 and $100.00"},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := newTestPortfolio("0").
				holding("ONLY", "30", testLot{"10", "30", 24}).
				asset("1", "ONLY")

			_, err := PlanSales(context.Background(), p.snapshot, p.quotes, testSaleDate, p.plan(testRequest(test.amount, "0", test.maxExcess, nil)))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %v, want %q", err, test.want)
			}
		})
	}
}

// Check the premise that lets the search stop a walk early: past a stop, no share count
// could have beaten the best plan. Every count in sampled branches is evaluated directly,
// since a wrong stop would only show up as a slightly worse plan.
func TestStoppingEarlySkipsOnlyLosingShareCounts(t *testing.T) {
	randomSeeds, realisticSeeds := int64(200), int64(3)
	if testing.Short() {
		randomSeeds, realisticSeeds = 50, 1
	}

	var cases []*testPortfolio
	var requests []models.LiquidationRequest
	for seed := range randomSeeds {
		if p, request, needsSale := randomCase(seed, wideShape); needsSale {
			cases, requests = append(cases, p), append(requests, request)
		}
	}
	for seed := range realisticSeeds {
		p, request := realisticCase(seed, "7000", "500", "0.02")
		cases, requests = append(cases, p), append(requests, request)
	}

	shortfallStops, taxStops := 0, 0
	for i, p := range cases {
		search, err := p.search(requests[i])
		if err != nil {
			continue
		}
		if err := search.run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if !search.found {
			continue
		}

		final := search.bestTotals
		random := rand.New(rand.NewSource(int64(i)))
		for _, incumbent := range []saleTotals{final, worsened(final, "1", "0"), worsened(final, "0", "25"), worsened(final, "100", "100")} {
			search.bestTotals = incumbent
			for range 30 {
				depth, soldSoFar, ok := randomBranch(search, random)
				if !ok {
					continue
				}
				shortfall, tax := checkStoppingRule(t, search, depth, soldSoFar)
				shortfallStops += shortfall
				taxStops += tax

				// Use incumbents from this branch so shortfall and tax tie exactly, as the
				// best plan often does when it comes from a neighbouring count.
				for _, extraProceeds := range []string{"0", "1000"} {
					tied, ok := tiedIncumbent(search, depth, soldSoFar, random, extraProceeds)
					if !ok {
						continue
					}
					search.bestTotals = tied
					shortfall, tax := checkStoppingRule(t, search, depth, soldSoFar)
					shortfallStops += shortfall
					taxStops += tax
					search.bestTotals = incumbent
				}
			}
		}
	}

	if shortfallStops == 0 || taxStops == 0 {
		t.Fatalf("stops by shortfall %d, by tax %d; the test must exercise both", shortfallStops, taxStops)
	}
}

func worsened(totals saleTotals, shortfall, tax string) saleTotals {
	totals.shortfallBelowMin = totals.shortfallBelowMin.Add(decimal.RequireFromString(shortfall))
	totals.tax = totals.tax.Add(decimal.RequireFromString(tax))
	return totals
}

// Take the best case of a random share count in the branch, so the incumbent's
// shortfall and tax are values the branch actually reaches.
func tiedIncumbent(search *saleSearch, depth int, soldSoFar saleTotals, random *rand.Rand, extraProceeds string) (saleTotals, bool) {
	fewest, most := search.shareCountRange(depth, soldSoFar.proceeds)
	if fewest > most {
		return saleTotals{}, false
	}

	shares := fewest + random.Int63n(most-fewest+1)
	tied := search.bestCase(depth+1, search.withShares(depth, soldSoFar, shares))
	tied.proceeds = tied.proceeds.Add(decimal.RequireFromString(extraProceeds))
	return tied, true
}

// Pick share counts for a random prefix of positions, staying within the ranges the
// search itself would try so the branch is one it could reach.
func randomBranch(search *saleSearch, random *rand.Rand) (int, saleTotals, bool) {
	depth := random.Intn(len(search.positions))
	var soldSoFar saleTotals
	for i := range depth {
		fewest, most := search.shareCountRange(i, soldSoFar.proceeds)
		if fewest > most {
			return 0, saleTotals{}, false
		}
		soldSoFar = search.withShares(i, soldSoFar, fewest+random.Int63n(most-fewest+1))
	}
	return depth, soldSoFar, true
}

// Walk both directions from the best starting count, as the search does, and fail if
// any count past a stop could beat the best plan.
func checkStoppingRule(t *testing.T, search *saleSearch, depth int, soldSoFar saleTotals) (shortfallStops, taxStops int) {
	t.Helper()

	fewest, most := search.shareCountRange(depth, soldSoFar.proceeds)
	if fewest > most {
		return 0, 0
	}
	start, err := search.bestStartingShares(context.Background(), depth, soldSoFar, fewest, most)
	if err != nil {
		t.Fatal(err)
	}
	bestCaseAt := func(shares int64) saleTotals {
		return search.bestCase(depth+1, search.withShares(depth, soldSoFar, shares))
	}

	for _, walk := range []struct{ first, last, step int64 }{{start, fewest, -1}, {start + 1, most, 1}} {
		for shares := walk.first; shares*walk.step <= walk.last*walk.step; shares += walk.step {
			bestCase := bestCaseAt(shares)
			if search.canBeatBest(bestCase) || !search.furtherSharesCannotBeatBest(bestCase) {
				continue
			}

			if bestCase.shortfallBelowMin.GreaterThan(search.bestTotals.shortfallBelowMin) {
				shortfallStops++
			} else {
				taxStops++
			}
			for further := shares + walk.step; further*walk.step <= walk.last*walk.step; further += walk.step {
				if search.canBeatBest(bestCaseAt(further)) {
					t.Fatalf("stopped at %d shares of %s, but %d shares could beat the best plan",
						shares, search.positions[depth].ticker, further)
				}
			}
			break
		}
	}
	return shortfallStops, taxStops
}
