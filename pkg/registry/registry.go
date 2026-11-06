package registry

import (
	"context"
	"fmt"
	"seesaw/internal/broker/etrade"
	"seesaw/pkg/models"
	"seesaw/pkg/secrets"
	"strings"
)

type Broker interface {
	Setup(context.Context) (saved bool, err error)
	Connect(context.Context) (models.BrokerClient, error)
}

// The broker registered under name, keeping its credentials and tokens in store.
func FindBroker(name string, store secrets.Store) (Broker, error) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case etrade.Name:
		return etrade.Broker{Secrets: store}, nil
	default:
		return nil, fmt.Errorf("unsupported broker %q", name)
	}
}
