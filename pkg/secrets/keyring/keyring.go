package keyring

import (
	"context"
	"errors"
	"fmt"
	"seesaw/pkg/secrets"

	oskeyring "github.com/zalando/go-keyring"
)

const service = "seesaw"

type Store struct{}

var _ secrets.Store = Store{}

func (Store) Get(_ context.Context, key string) ([]byte, error) {
	data, err := oskeyring.Get(service, key)
	if errors.Is(err, oskeyring.ErrNotFound) {
		return nil, secrets.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("OS keyring: failed to get %s: %w", key, err)
	}
	return []byte(data), nil
}

func (Store) Set(_ context.Context, key string, value []byte) error {
	if err := oskeyring.Set(service, key, string(value)); err != nil {
		return fmt.Errorf("OS keyring: failed to set %s: %w", key, err)
	}
	return nil
}
