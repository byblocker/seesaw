package cmd

import (
	"os"
	"seesaw/pkg"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "seesaw",
	Short: "Seesaw: An investment portfolio rebalancing engine",
	Run: func(cmd *cobra.Command, args []string) {
		pkg.Rebalance()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {

}
