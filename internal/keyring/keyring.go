package keyring

import (
	"encoding/json"
	"errors"
	"fmt"

	oskeyring "github.com/zalando/go-keyring"
)

const service = "seesaw"

var ErrNotFound = oskeyring.ErrNotFound

func Set[T any](key string, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	return oskeyring.Set(service, key, string(data))
}

func Get[T any](key string) (*T, error) {
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

func Delete(key string) error {
	return oskeyring.Delete(service, key)
}

func Exists(key string) (bool, error) {
	_, err := oskeyring.Get(service, key)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check key %s: %w", key, err)
	}
	return true, nil
}
