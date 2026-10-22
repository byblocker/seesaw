package cmd

import (
	"path/filepath"
	"seesaw/pkg/config"
)

const (
	portfolioDir   = "./data/portfolios"
	userConfigPath = "./data/user.yaml"
)

func loadPlan(name string) (*config.Plan, error) {
	fileName := name + ".yaml"
	pathToPlan := filepath.Join(portfolioDir, fileName)
	return config.LoadPlan(pathToPlan)
}

func loadTaxRates(plan *config.Plan) (*config.TaxSettings, error) {
	if !*plan.IsTaxable {
		return nil, nil
	}

	user, err := config.LoadUser(userConfigPath)
	if err != nil {
		return nil, err
	}
	return &user.Tax, nil
}
