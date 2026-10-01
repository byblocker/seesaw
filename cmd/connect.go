package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"seesaw/pkg"

	"github.com/spf13/cobra"
)

var (
	brokerName string

	connectCmd = &cobra.Command{
		Use:   "connect",
		Short: "Configure and authenticate a broker",
		Args:  cobra.NoArgs,
		RunE:  connectBroker,
	}
)

func init() {
	connectCmd.Flags().StringVar(&brokerName, "broker", "", "broker to connect (ETRADE)")
	if err := connectCmd.MarkFlagRequired("broker"); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(connectCmd)
}

func connectBroker(cmd *cobra.Command, args []string) error {
	provider, err := pkg.Provider(brokerName)
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	if err := provider.Setup(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Print("Connection setup canceled; configuration unchanged.")
			return nil
		}
		return err
	}

	if _, err := provider.Connect(ctx); err != nil {
		return fmt.Errorf("configuration saved, but authentication failed; retry with rebalance or rerun connect to replace the configuration: %w", err)
	}

	log.Print("Broker connected.")
	return nil
}
