package cmd

import (
	"context"
	"fmt"
	"seesaw/internal/broker/etrade"
	"seesaw/pkg/models"
	"seesaw/pkg/secrets/keyring"
	"strings"
)

type broker interface {
	Setup(context.Context) (saved bool, err error)
	Connect(context.Context) (models.BrokerClient, error)
}

func findBroker(name string) (broker, error) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case etrade.Name:
		return etrade.Broker{Secrets: keyring.Store{}}, nil
	default:
		return nil, fmt.Errorf("unsupported broker %q", name)
	}
}
