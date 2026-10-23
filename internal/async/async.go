package async

import (
	"context"
	"maps"
	"slices"

	"golang.org/x/sync/errgroup"
)

// Call fn for every key at once, and map each key to its result. The first error cancels the rest.
func ForKeys[K comparable, R any](ctx context.Context, keys []K, fn func(context.Context, K) (R, error)) (map[K]R, error) {
	results := make([]R, len(keys))
	group, groupCtx := errgroup.WithContext(ctx)
	for i, key := range keys {
		group.Go(func() error {
			result, err := fn(groupCtx, key)
			results[i] = result
			return err
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	byKey := make(map[K]R, len(keys))
	for i, key := range keys {
		byKey[key] = results[i]
	}
	return byKey, nil
}

// Call fn for every value at once, and map each value's key to its result. The first error cancels the rest.
func ForValues[K comparable, V, R any](ctx context.Context, items map[K]V, fn func(context.Context, V) (R, error)) (map[K]R, error) {
	return ForKeys(ctx, slices.Collect(maps.Keys(items)), func(ctx context.Context, key K) (R, error) {
		return fn(ctx, items[key])
	})
}
