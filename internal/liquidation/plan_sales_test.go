package liquidation

import (
	"context"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

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
