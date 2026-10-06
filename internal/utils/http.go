package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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

func logRequestAttempt(_ *resty.Client, request *resty.Request) error {
	writer := log.Writer()
	request.SetLogger(requestLogger{writer: writer})
	if request.Attempt > 1 && request.Context().Err() == nil {
		fmt.Fprintf(writer, "attempt %d failed; ", request.Attempt-1)
	}
	return nil
}

// Install this logger when execution begins so configuration errors keep their
// original logger. Execution errors are returned to the caller for final reporting.
type requestLogger struct {
	writer io.Writer
}

var _ resty.Logger = requestLogger{}

func (logger requestLogger) Warnf(format string, arguments ...any) {
	// Resty emits this warning before deciding whether another attempt will run.
	// Report actual retries through request metadata instead of its warning text.
	if format == "%v, Attempt %v" {
		return
	}
	message := fmt.Sprintf(format, arguments...)
	fmt.Fprintf(logger.writer, "WARNING: %s; ", strings.Join(strings.Fields(message), " "))
}

func (requestLogger) Errorf(string, ...any) {
	// Resty's final execution error is also returned to the caller.
}

func (requestLogger) Debugf(format string, arguments ...any) {
	log.Printf(format, arguments...)
}
