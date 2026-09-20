package timing

import (
	"context"
	"fmt"
	"time"
)

type RetryTimer struct {
	// Stores a schedule of delays (ms) for tiemouts between API attempts; an empty
	// schedule allows a single, immediate attempt; non-postive delays are immediate;
	// the first delay occurs before any attempt is made, and therefore should be 0
	// in most cases.
	delays []time.Duration
}

func NewRetryTimer(milliseconds []int) RetryTimer {
	delays := make([]time.Duration, max(1, len(milliseconds)))
	for i, ms := range milliseconds {
		delays[i] = time.Duration(ms) * time.Millisecond
	}

	return RetryTimer{delays: delays}
}

func (t RetryTimer) Attempts() int {
	return max(1, len(t.delays))
}

func (t RetryTimer) Wait(ctx context.Context, attempt int) error {
	if attempt < 1 || attempt > t.Attempts() {
		return fmt.Errorf("retry attempt %d outside range 1..%d", attempt, t.Attempts())
	}

	if len(t.delays) == 0 {
		return Sleep(ctx, 0)
	}

	return Sleep(ctx, t.delays[attempt-1])
}
