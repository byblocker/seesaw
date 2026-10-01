package broker

import (
	"context"
	"fmt"
	"seesaw/internal/broker/etrade"
	"seesaw/pkg/models"
	"strings"
)

var ErrNotConfigured = models.ErrBrokerNotConfigured

type Broker interface {
	Setup(context.Context) error
	Connect(context.Context) (models.BrokerClient, error)
}

func For(name string) (Broker, error) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case etrade.Name:
		return etrade.Broker{}, nil
	default:
		return nil, fmt.Errorf("unsupported broker %q", name)
	}
}
