package secrets

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("secret not found")

// Where brokers keep their credentials and tokens. The CLI uses the OS keyring;
// other hosts, such as a Lambda function, can back it with a cloud secret manager.
type Store interface {
	// Get returns ErrNotFound when nothing is saved under key.
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte) error
}

func Exists(ctx context.Context, store Store, key string) (bool, error) {
	_, err := store.Get(ctx, key)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}
