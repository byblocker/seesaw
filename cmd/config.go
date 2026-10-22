package cmd

import (
	"fmt"
	"path/filepath"
	"seesaw/pkg/config"
	"strings"
)

const (
	portfolioDir   = "./data/portfolios"
	userConfigPath = "./data/user.yaml"
)

func loadPlan(name string) (*config.Plan, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("--portfolio must not be empty")
	}

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
