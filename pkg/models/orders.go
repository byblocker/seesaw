package models

import (
	"log"
	"seesaw/internal/utils"
	"seesaw/pkg/config"
	"time"

	"github.com/shopspring/decimal"
)

type Buy struct {
	Ticker        string
	Price         decimal.Decimal
	Quantity      decimal.Decimal
	EstimatedCost decimal.Decimal
}

type SellProposal struct {
	Ticker     string
	Price      decimal.Decimal
	Lots       []LotSale
	Proceeds   decimal.Decimal
	RealizedPL decimal.Decimal
}

func GroupSales(sales []LotSale) []SellProposal {
	var proposals []SellProposal
	proposalByTicker := map[string]int{}

	for _, sale := range sales {
		i, exists := proposalByTicker[sale.Ticker]
		if !exists {
			i = len(proposals)
			proposalByTicker[sale.Ticker] = i
			proposals = append(proposals, SellProposal{Ticker: sale.Ticker, Price: sale.Price})
		}

		proposals[i].Add(sale)
	}

	return proposals
}

func (p *SellProposal) Add(sale LotSale) {
	p.Lots = append(p.Lots, sale)
	p.Proceeds = p.Proceeds.Add(sale.Proceeds())
	p.RealizedPL = p.RealizedPL.Add(sale.RealizedPL())
}

func (p SellProposal) IsEmpty() bool {
	return len(p.Lots) == 0
}

func (p SellProposal) HarvestsAtLeast(other SellProposal) bool {
	return p.RealizedPL.LessThanOrEqual(other.RealizedPL)
}

func (p SellProposal) LogProceeds() {
	log.Printf("Estimated %s proceeds from selling %s", utils.FormatMoney(p.Proceeds), p.Ticker)
}

type LotSale struct {
	Ticker       string
	Quantity     decimal.Decimal
	LotIdx       int
	Price        decimal.Decimal
	GainPerShare decimal.Decimal
	GainPct      decimal.Decimal
	LongTerm     bool
	IsWashSale   bool
}

func NewLotSale(
	ticker string,
	lotIdx int,
	lot PositionLot,
	price decimal.Decimal,
	saleDate time.Time,
	isRecentlyBought bool,
) LotSale {
	gainPerShare := price.Sub(lot.PricePaid)
	isWashSale := isRecentlyBought && gainPerShare.IsNegative()
	gainPct := decimal.Zero
	if lot.PricePaid.IsPositive() {
		gainPct, _ = gainPerShare.QuoRem(lot.PricePaid, utils.DivisionDecimalPlaces)
	}

	return LotSale{
		Ticker: ticker, LotIdx: lotIdx, Price: price,
		Quantity:     lot.Quantity,
		GainPerShare: gainPerShare,
		GainPct:      gainPct,
		LongTerm:     lot.IsLongTermAt(saleDate),
		IsWashSale:   isWashSale,
	}
}

// Zero without rates, as for an account that is not taxable.
func (sale LotSale) TaxImpactPerShare(rates *config.TaxSettings) decimal.Decimal {
	if rates == nil || sale.IsWashSale {
		return decimal.Zero
	}

	rate := rates.ShortTermGainRate
	if sale.LongTerm {
		rate = rates.LongTermGainRate
	}
	return sale.GainPerShare.Mul(rate)
}

// Compare tax per dollar raised exactly across share prices by scaling each tax per share with the other price.
func CompareTaxPerDollar(leftTaxPerShare, leftPrice, rightTaxPerShare, rightPrice decimal.Decimal) int {
	return leftTaxPerShare.Mul(rightPrice).Cmp(rightTaxPerShare.Mul(leftPrice))
}

func (sale LotSale) Proceeds() decimal.Decimal {
	return sale.Quantity.Mul(sale.Price)
}

func (sale LotSale) RealizedPL() decimal.Decimal {
	return sale.Quantity.Mul(sale.GainPerShare)
}

func (sale LotSale) EstimatedTaxImpact(rates config.TaxSettings) decimal.Decimal {
	return sale.Quantity.Mul(sale.TaxImpactPerShare(&rates))
}
