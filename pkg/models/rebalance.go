package models

type RebalancePlan struct {
	Buys  []Buy
	Sells []SellProposal
}
