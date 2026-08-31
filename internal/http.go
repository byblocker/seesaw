package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func SendHTTPRequest(
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

func SendJSONRequest[T any](
	ctx context.Context,
	client *http.Client,
	method, endpoint string,
	headers http.Header,
	query url.Values,
) (T, error) {
	var response T

	if headers == nil {
		headers = http.Header{}
	}
	headers.Add("Accept", "application/json")

	body, err := SendHTTPRequest(ctx, client, method, endpoint, headers, query)
	if err != nil {
		return response, err
	}
	if len(body) == 0 {
		return response, nil
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return response, fmt.Errorf("error unmarshaling JSON: %w", err)
	}
	return response, nil
}
