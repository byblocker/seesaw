package models

import "seesaw/pkg/config"

type RebalanceRequest struct {
	Settings config.Settings
	TaxRates *config.TaxSettings // Nil for non-taxable accounts
	Force    bool
}

func (request RebalanceRequest) Validate() error {
	if err := request.Settings.Validate(); err != nil {
		return err
	}
	if request.TaxRates != nil {
		return request.TaxRates.Validate()
	}
	return nil
}

type RebalancePlan struct {
	Buys  []Buy
	Sells []SellProposal
}
