package async

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Call fn for every item at once; results keep the items' order. The first error cancels the rest.
func Map[T, R any](ctx context.Context, items []T, fn func(context.Context, T) (R, error)) ([]R, error) {
	results := make([]R, len(items))
	group, groupCtx := errgroup.WithContext(ctx)
	for i, item := range items {
		group.Go(func() error {
			result, err := fn(groupCtx, item)
			results[i] = result
			return err
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// Like Map, but returns each key's result in a map.
func MapByKey[K comparable, R any](ctx context.Context, keys []K, fn func(context.Context, K) (R, error)) (map[K]R, error) {
	results, err := Map(ctx, keys, fn)
	if err != nil {
		return nil, err
	}

	byKey := make(map[K]R, len(keys))
	for i, key := range keys {
		byKey[key] = results[i]
	}
	return byKey, nil
}
