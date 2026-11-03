package models

import (
	"log"
	"seesaw/internal/utils"

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
