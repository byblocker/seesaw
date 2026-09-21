package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"seesaw/internal/timing"
	"syscall"
	"time"
)

const (
	maxHTTPResponseBytes int64         = 10 * 1024 * 1024
	maxHTTPErrorBytes    int64         = 4 * 1024
	httpRequestTimeout   time.Duration = 30 * time.Second
)

// Milliseconds before each HTTP attempt. {0, 250, 500} allows two short retries
// after an immediate attempt to recover from transient GET connection failures.
var msPerRetryAPI = []int{0, 250, 500}

func SendHTTPRequest(
	ctx context.Context,
	client *http.Client,
	method, endpoint string,
	headers http.Header,
) ([]byte, error) {
	timer := timing.NewRetryTimer(msPerRetryAPI)
	for attempt := 1; ; attempt++ {
		if err := timer.Wait(ctx, attempt); err != nil {
			return nil, fmt.Errorf("wait before request: %w", err)
		}
		body, err := sendHTTPRequestOnce(ctx, client, method, endpoint, headers)
		if err == nil {
			return body, nil
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request canceled: %w", ctx.Err())
		}
		if method != http.MethodGet || !isTransientError(err) {
			return nil, err
		}
		if attempt == timer.Attempts() {
			return nil, fmt.Errorf("request failed after %d attempts: %w", attempt, err)
		}
	}
}

func sendHTTPRequestOnce(
	ctx context.Context,
	client *http.Client,
	method, endpoint string,
	headers http.Header,
) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, httpRequestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if headers != nil {
		request.Header = headers.Clone()
	}

	resp, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("unauthorized request")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		excerpt, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPErrorBytes+1))
		if err != nil {
			return nil, fmt.Errorf("unexpected status %d: failed to read response body: %w", resp.StatusCode, err)
		}
		if int64(len(excerpt)) > maxHTTPErrorBytes {
			return nil, fmt.Errorf("unexpected status %d: %s [truncated]", resp.StatusCode, excerpt[:maxHTTPErrorBytes])
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, excerpt)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if int64(len(body)) > maxHTTPResponseBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxHTTPResponseBytes)
	}
	return body, nil
}

func isTransientError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var networkError net.Error
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) ||
		(errors.As(err, &networkError) && networkError.Timeout())
}
