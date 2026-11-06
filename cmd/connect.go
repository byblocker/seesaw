package cmd

import (
	"fmt"
	"seesaw/pkg/registry"
	"seesaw/pkg/secrets/keyring"

	"github.com/spf13/cobra"
)

var (
	brokerName string

	connectCmd = &cobra.Command{
		Use:     "connect",
		Short:   "Configure and authenticate a broker",
		Example: "  seesaw connect --broker ETRADE",
		Args:    cobra.NoArgs,
		RunE:    connectBroker,
	}
)

func init() {
	connectCmd.Flags().StringVar(&brokerName, "broker", "", "broker to connect (ETRADE)")
	if err := connectCmd.MarkFlagRequired("broker"); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(connectCmd)
}

// Selects and configures a Broker before an authenticated BrokerClient exists.
func connectBroker(cmd *cobra.Command, args []string) error {
	selectedBroker, err := registry.FindBroker(brokerName, keyring.Store{})
	if err != nil {
		return err
	}

	saved, err := selectedBroker.Setup(cmd.Context())
	if err != nil {
		return err
	}
	if !saved {
		fmt.Fprintln(cmd.ErrOrStderr(), "Connection setup canceled; configuration unchanged.")
		return nil
	}

	if _, err := selectedBroker.Connect(cmd.Context()); err != nil {
		return fmt.Errorf("configuration saved, but authentication failed: %w\n"+
			"To retry sign-in, run: seesaw snapshot <portfolio>\n"+
			"To replace the configuration, run: seesaw connect --broker %s", err, brokerName)
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "Broker connected.")
	return nil
}
