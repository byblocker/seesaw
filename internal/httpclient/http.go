package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"seesaw/internal/config"
	"seesaw/internal/timing"
	"syscall"
)

func SendHTTPRequest(
	ctx context.Context,
	client *http.Client,
	method, endpoint string,
	headers http.Header,
	query url.Values,
) ([]byte, error) {
	timer := timing.NewRetryTimer(config.MsPerRetryAPI)
	for attempt := 1; ; attempt++ {
		if err := timer.Wait(ctx, attempt); err != nil {
			return nil, fmt.Errorf("wait before request: %w", err)
		}
		body, err := sendHTTPRequestOnce(ctx, client, method, endpoint, headers, query)
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
	query url.Values,
) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if headers != nil {
		request.Header = headers.Clone()
	}
	if query != nil {
		request.URL.RawQuery = query.Encode()
	}

	resp, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, errors.New("unauthorized request")
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	default:
		return body, nil
	}
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
