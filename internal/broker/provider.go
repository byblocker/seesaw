package broker

import (
	"context"
	"fmt"
	"seesaw/internal/broker/etrade"
	"seesaw/pkg/models"
	"strings"
)

var ErrNotConfigured = models.ErrBrokerNotConfigured

type Provider interface {
	Setup(context.Context) error
	Connect(context.Context) (models.Broker, error)
}

func For(name string) (Provider, error) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case etrade.Name:
		return etrade.Provider{}, nil
	default:
		return nil, fmt.Errorf("unsupported broker %q", name)
	}
}
