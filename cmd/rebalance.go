package cmd

import (
	"log"
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
	log.Print("Starting rebalance . . .")
	snapshot, err := pkg.SnapshotPortfolio(portfolioPlan, 0)
	if err != nil {
		return err
	}

	_ = snapshot
	return nil
}
