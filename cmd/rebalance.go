package cmd

import (
	"seesaw/pkg"

	"github.com/spf13/cobra"
)

var rebalancePortfolio = &cobra.Command{
	Use:   "rebalance",
	Short: "Rebalance portfolio according to portfolio plan",
	RunE:  rebalance,
}

func init() {
	rootCmd.AddCommand(rebalancePortfolio)
}

func rebalance(cmd *cobra.Command, args []string) error {
	portfolio, err := pkg.GetPortfolio(portfolioPlan)
	if err != nil {
		return err
	}

	_ = portfolio
	return nil
}
