package models

import "github.com/shopspring/decimal"

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
	p.Proceeds = p.Proceeds.Add(sell.Quantity.Mul(p.Price))
	p.RealizedPL = p.RealizedPL.Add(sell.Quantity.Mul(p.Price.Sub(sell.Lot.PricePaid)))
}

type SellLot struct {
	Lot      PositionLot
	Quantity decimal.Decimal
}
