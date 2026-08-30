package cmd

import (
	"os"
	"seesaw/pkg/services"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "seesaw",
	Short: "Seesaw: An investment portfolio rebalancing engine",
	Run: func(cmd *cobra.Command, args []string) {
		_ = services.LoadEnv()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {

}
