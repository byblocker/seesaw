package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"seesaw/pkg/config"
	"strings"

	"github.com/spf13/cobra"
)

const (
	portfolioDir   = "data/portfolios"
	userConfigPath = "data/user.yaml"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:           "seesaw",
	Short:         "Seesaw: An investment portfolio rebalancing engine",
	SilenceErrors: true,
	SilenceUsage:  true,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if !verbose {
			log.SetOutput(io.Discard)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show progress logs")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadPlan(name string) (*config.Plan, error) {
	plan, err := config.LoadPlan(planPath(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, planChoiceError(name, fmt.Sprintf("no portfolio plan named %q", name))
	}
	return plan, err
}

// Requires the portfolio argument, listing the plans to choose from when it is absent.
func requirePlanName(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return planChoiceError("<name>", "must specify a supported portfolio")
	}
	return planNameArg(cmd, args)
}

func planNameArg(cmd *cobra.Command, args []string) error {
	if err := cobra.ExactArgs(1)(cmd, args); err != nil {
		return err
	}
	if strings.TrimSpace(args[0]) == "" {
		return fmt.Errorf("portfolio name must not be empty")
	}
	return nil
}

// The problem followed by the plans to choose from, or how to create one when there are none.
func planChoiceError(name, problem string) error {
	names := planNames()
	if len(names) == 0 {
		return fmt.Errorf("no portfolio plans in %s. To create one:\n"+
			"  1. Run: seesaw init %s\n"+
			"  2. Edit %s and %s", portfolioDir, name, planPath(name), userConfigPath)
	}
	return fmt.Errorf("%s: %s", problem, strings.Join(names, ", "))
}

func planPath(name string) string {
	return filepath.Join(portfolioDir, name+".yaml")
}

func planNames() []string {
	paths, _ := filepath.Glob(filepath.Join(portfolioDir, "*.yaml"))
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, strings.TrimSuffix(filepath.Base(path), ".yaml"))
	}
	return names
}

// The user's tax rates for the plan's account, or nil when the account is not taxable.
func loadTaxRates(plan *config.Plan) (*config.TaxSettings, error) {
	user, err := config.LoadUser(userConfigPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no user config at %s. To create one:\n"+
			"  1. Run: cp %s %s\n"+
			"  2. Edit %s", userConfigPath, filepath.Join(examplesDir, "user.yaml"), userConfigPath, userConfigPath)
	}
	if err != nil {
		return nil, err
	}
	return user.TaxRates(plan.IsTaxable)
}
