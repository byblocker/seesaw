package utils

import "context"

type verboseContextKey struct{}

func WithVerbose(ctx context.Context) context.Context {
	return context.WithValue(ctx, verboseContextKey{}, true)
}

func IsVerbose(ctx context.Context) bool {
	verbose, _ := ctx.Value(verboseContextKey{}).(bool)
	return verbose
}
