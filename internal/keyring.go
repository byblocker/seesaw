package internal

import (
	"encoding/json"
	"fmt"

	"github.com/zalando/go-keyring"
)

func KeyringSet[T any](service, key string, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	return keyring.Set(service, key, string(data))
}

func KeyringGet[T any](service, key string) (*T, error) {
	data, err := keyring.Get(service, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}

	var value T
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key %s: %w", key, err)
	}

	return &value, nil
}

func KeyringDelete(service, key string) error {
	return keyring.Delete(service, key)
}
