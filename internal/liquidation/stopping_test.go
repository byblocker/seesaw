package liquidation

import (
	"context"
	"math/rand"
	"testing"

	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

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
