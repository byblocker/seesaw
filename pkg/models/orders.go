package models

import (
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

type LotSale struct {
	Ticker       string
	Quantity     decimal.Decimal
	Lot          *PositionLot
	Price        decimal.Decimal
	GainPerShare decimal.Decimal
	LongTerm     bool
	IsWashSale   bool
}

func NewLotSale(
	ticker string,
	lot *PositionLot,
	price decimal.Decimal,
	saleDate time.Time,
	isRecentlyBought bool,
) LotSale {
	gainPerShare := price.Sub(lot.PricePaid)
	isWashSale := isRecentlyBought && gainPerShare.IsNegative()

	return LotSale{
		Ticker: ticker, Lot: lot, Price: price,
		Quantity:     lot.Quantity,
		GainPerShare: gainPerShare,
		LongTerm:     lot.IsLongTermAt(saleDate),
		IsWashSale:   isWashSale,
	}
}

func (sale LotSale) TaxImpactPerShare(rates config.TaxSettings) decimal.Decimal {
	if sale.IsWashSale {
		return decimal.Zero
	}

	rate := rates.ShortTermGainRate
	if sale.LongTerm {
		rate = rates.LongTermGainRate
	}
	return sale.GainPerShare.Mul(rate)
}

func (sale LotSale) Proceeds() decimal.Decimal {
	return sale.Quantity.Mul(sale.Price)
}

func (sale LotSale) RealizedPL() decimal.Decimal {
	return sale.Quantity.Mul(sale.GainPerShare)
}

// Gain or loss at the sale price as a fraction of the price paid.
func (sale LotSale) GainPct() decimal.Decimal {
	if !sale.Lot.PricePaid.IsPositive() {
		return decimal.Zero
	}

	gainPct, _ := sale.GainPerShare.QuoRem(sale.Lot.PricePaid, utils.DivisionDecimalPlaces)
	return gainPct
}

func (sale LotSale) EstimatedTaxImpact(rates config.TaxSettings) decimal.Decimal {
	return sale.Quantity.Mul(sale.TaxImpactPerShare(rates))
}
