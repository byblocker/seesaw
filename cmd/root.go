package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"seesaw/pkg"

	"github.com/spf13/cobra"
)

const portfolioDir = "./data/portfolios"

var (
	portfolioPlanName string
	portfolioPlan     *pkg.PortfolioPlan
)

var rootCmd = &cobra.Command{
	Use:   "seesaw",
	Short: "Seesaw: An investment portfolio rebalancing engine",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		pathToPlan := filepath.Join(portfolioDir, portfolioPlanName+".yaml")
		file, err := os.Open(pathToPlan)
		if err != nil {
			return fmt.Errorf("opening portfolio plan file %s: %w", pathToPlan, err)
		}
		defer file.Close()

		p, err := pkg.ParsePortfolioPlan(file)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		portfolioPlan = p
		return nil
	},
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
}

func init() {
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.PersistentFlags().StringVar(&portfolioPlanName, "portfolio", "", "name of portfolio plan")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
