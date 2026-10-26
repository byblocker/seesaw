package cmd

import (
	"errors"
	"fmt"
	"io/fs"
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
		return nil, fmt.Errorf("portfolio name must not be empty")
	}

	fileName := name + ".yaml"
	pathToPlan := filepath.Join(portfolioDir, fileName)
	plan, err := config.LoadPlan(pathToPlan)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, missingPlanError(name)
	}
	return plan, err
}

// Requires the portfolio argument, listing the plans to choose from when it is absent.
func requirePlanName(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cobra.ExactArgs(1)(cmd, args)
	}
	names := planNames()
	if len(names) == 0 {
		return noPlansError("<name>")
	}
	return fmt.Errorf("must specify a supported portfolio: %s", strings.Join(names, ", "))
}

func missingPlanError(name string) error {
	names := planNames()
	if len(names) == 0 {
		return noPlansError(name)
	}
	return fmt.Errorf("no portfolio plan named %q: %s", name, strings.Join(names, ", "))
}

func noPlansError(name string) error {
	planPath := filepath.Join(portfolioDir, name+".yaml")
	return fmt.Errorf("no portfolio plans in %s. To create one:\n"+
		"  1. Run: seesaw init %s\n"+
		"  2. Edit %s and %s", portfolioDir, name, planPath, userConfigPath)
}

func planNames() []string {
	paths, _ := filepath.Glob(filepath.Join(portfolioDir, "*.yaml"))
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, strings.TrimSuffix(filepath.Base(path), ".yaml"))
	}
	return names
}

func taxRates(plan *config.Plan, user *config.User) *config.TaxSettings {
	if !plan.IsTaxable {
		return nil
	}
	return &user.Tax
}
