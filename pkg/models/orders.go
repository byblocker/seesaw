package models

import (
	"fmt"

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
	p.Proceeds = p.Proceeds.Add(sell.Proceeds(p.Price))
	p.RealizedPL = p.RealizedPL.Add(sell.RealizedPL(p.Price))
}

type SellLot struct {
	Lot      PositionLot
	Quantity decimal.Decimal
}

func (sell SellLot) Proceeds(price decimal.Decimal) decimal.Decimal {
	return sell.Quantity.Mul(price)
}

func (sell SellLot) RealizedPL(price decimal.Decimal) decimal.Decimal {
	return sell.Quantity.Mul(price.Sub(sell.Lot.PricePaid))
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
