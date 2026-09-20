package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

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
		return response, debugDecodeError(err)
	}
	return response, nil
}

// Adds the location supplied by the encoding error
func debugDecodeError(cause error) error {
	var syntax *json.SyntaxError
	if errors.As(cause, &syntax) {
		return fmt.Errorf("decode JSON response at byte %d: %w", syntax.Offset, cause)
	}

	var mismatch *json.UnmarshalTypeError
	if errors.As(cause, &mismatch) {
		if mismatch.Field != "" {
			return fmt.Errorf("decode JSON response field %q at byte %d: %w", mismatch.Field, mismatch.Offset, cause)
		}
		return fmt.Errorf("decode JSON response at byte %d: %w", mismatch.Offset, cause)
	}
	return fmt.Errorf("decode JSON response: %w", cause)
}
