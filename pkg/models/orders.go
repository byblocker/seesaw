package models

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type OrderType string

const (
	Market OrderType = "MARKET"
	Limit  OrderType = "LIMIT"
)

type Buy struct {
	Ticker        string
	Price         decimal.Decimal
	Quantity      decimal.Decimal
	EstimatedCost decimal.Decimal
}

type SellProposal struct {
	Ticker     string
	Type       OrderType
	Price      decimal.Decimal
	Lots       []SellLot
	Proceeds   decimal.Decimal
	RealizedPL decimal.Decimal
}

func (p *SellProposal) Add(sell SellLot) {
	p.Lots = append(p.Lots, sell)
	p.Proceeds = p.Proceeds.Add(sell.Proceeds())
	p.RealizedPL = p.RealizedPL.Add(sell.RealizedPL())
}

type SaleCandidate struct {
	Ticker       string
	Lot          *PositionLot
	Price        decimal.Decimal
	GainPerShare decimal.Decimal
	LongTerm     bool
}

func NewSaleCandidate(ticker string, lot *PositionLot, price decimal.Decimal, saleDate time.Time) SaleCandidate {
	return SaleCandidate{
		Ticker: ticker, Lot: lot, Price: price,
		GainPerShare: price.Sub(lot.PricePaid),
		LongTerm:     lot.IsLongTermAt(saleDate),
	}
}

func (sale SaleCandidate) TaxImpactPerShare(rates TaxSettings) decimal.Decimal {
	rate := rates.ShortTermGainRate
	if sale.LongTerm {
		rate = rates.LongTermGainRate
	}
	return sale.GainPerShare.Mul(*rate)
}

type SellLot struct {
	SaleCandidate
	Quantity decimal.Decimal
}

func (sell SellLot) Proceeds() decimal.Decimal {
	return sell.Quantity.Mul(sell.Price)
}

func (sell SellLot) RealizedPL() decimal.Decimal {
	return sell.Quantity.Mul(sell.GainPerShare)
}

func (sell SellLot) EstimatedTaxImpact(rates TaxSettings) decimal.Decimal {
	return sell.Quantity.Mul(sell.TaxImpactPerShare(rates))
}

func (proposal SellProposal) Validate() error {
	if !proposal.Price.IsPositive() {
		return fmt.Errorf("sale price must be positive")
	}
	for _, sell := range proposal.Lots {
		if !sell.Quantity.IsPositive() || sell.Quantity.GreaterThan(sell.Lot.Quantity) {
			return fmt.Errorf("sale quantity must be positive and no greater than lot quantity")
		}
	}
	return nil
}
