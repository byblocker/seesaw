package keyring

import (
	"encoding/json"
	"fmt"

	oskeyring "github.com/zalando/go-keyring"
)

func Set[T any](service, key string, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	return oskeyring.Set(service, key, string(data))
}

func Get[T any](service, key string) (*T, error) {
	data, err := oskeyring.Get(service, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}

	var value T
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key %s: %w", key, err)
	}

	return &value, nil
}

func Delete(service, key string) error {
	return oskeyring.Delete(service, key)
}
