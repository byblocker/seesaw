package async

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Call fn for every key at once, and return the results in key order. The first error cancels the rest.
func Map[K, R any](ctx context.Context, keys []K, fn func(context.Context, K) (R, error)) ([]R, error) {
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
	return results, nil
}
