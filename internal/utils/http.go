package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

const Timeout = 30 * time.Second
const RetryCount = 2
const ResponseBodyLimit = 10 * 1024 * 1024

func NewHTTPClient(client *http.Client) *resty.Client {
	return resty.NewWithClient(client).
		SetTimeout(Timeout).
		SetRetryCount(RetryCount).
		SetResponseBodyLimit(ResponseBodyLimit).
		OnBeforeRequest(logRequestAttempt).
		SetJSONUnmarshaler(func(body []byte, response any) error {
			if len(body) == 0 {
				return nil
			}
			return json.Unmarshal(body, response)
		}).
		OnAfterResponse(func(_ *resty.Client, response *resty.Response) error {
			if response.IsSuccess() {
				return nil
			}
			const maxErrorBytes = 4 * 1024
			excerpt := response.Body()
			if len(excerpt) > maxErrorBytes {
				return fmt.Errorf(
					"unexpected status %d: %s [truncated]",
					response.StatusCode(),
					excerpt[:maxErrorBytes],
				)
			}
			return fmt.Errorf("unexpected status %d: %s", response.StatusCode(), excerpt)
		})
}

func FetchJSON[T any](ctx context.Context, client *resty.Client, endpoint string) (T, error) {
	var response T
	_, err := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		ForceContentType("application/json").
		SetResult(&response).
		Get(endpoint)
	return response, err
}
