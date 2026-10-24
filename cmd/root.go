package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"seesaw/pkg/config"
	"strings"

	"github.com/spf13/cobra"
)

const (
	portfolioDir   = "./data/portfolios"
	userConfigPath = "./data/user.yaml"
)

var rootCmd = &cobra.Command{
	Use:           "seesaw",
	Short:         "Seesaw: An investment portfolio rebalancing engine",
	SilenceErrors: true,
	SilenceUsage:  true,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
}

func init() {
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadPlan(name string) (*config.Plan, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("--portfolio must not be empty")
	}

	fileName := name + ".yaml"
	pathToPlan := filepath.Join(portfolioDir, fileName)
	return config.LoadPlan(pathToPlan)
}

func taxRates(plan *config.Plan, user *config.User) *config.TaxSettings {
	if !plan.IsTaxable {
		return nil
	}
	return &user.Tax
}
