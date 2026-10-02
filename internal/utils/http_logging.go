package utils

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/go-resty/resty/v2"
)

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
