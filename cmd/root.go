package cmd

import (
	"fmt"
	"log"
	"os"
	"seesaw/internal/utils"

	"github.com/spf13/cobra"
)

const (
	portfolioDir   = "./data/portfolios"
	userConfigPath = "./data/user.yaml"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:           "seesaw",
	Short:         "Seesaw: An investment portfolio rebalancing engine",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		flags := log.Flags() &^ log.Lshortfile
		if verbose {
			flags |= log.Lshortfile
			cmd.SetContext(utils.WithVerbose(cmd.Context()))
		}
		log.SetFlags(flags)
	},
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
}

func init() {
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show source locations, transaction pages, and rebalance decisions")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
