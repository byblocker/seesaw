package utils

import (
	"context"
	"fmt"
	"log"
)

type verboseContextKey struct{}

func WithVerbose(ctx context.Context) context.Context {
	return context.WithValue(ctx, verboseContextKey{}, true)
}

func IsVerbose(ctx context.Context) bool {
	verbose, _ := ctx.Value(verboseContextKey{}).(bool)
	return verbose
}

func Verbosef(ctx context.Context, format string, arguments ...any) {
	if IsVerbose(ctx) {
		log.Output(2, fmt.Sprintf(format, arguments...))
	}
}
