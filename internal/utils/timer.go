package utils

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

func StartTimer(format string, arguments ...any) (stop func(err error, details ...string)) {
	started := time.Now()
	writer := log.Writer()
	logger := log.New(actionWriter{writer}, log.Prefix(), log.Flags())
	logger.Output(2, fmt.Sprintf(format, arguments...)+"... ")

	return func(err error, details ...string) {
		elapsed := time.Since(started).Milliseconds()
		if err != nil {
			fmt.Fprintf(writer, "FAILED: %s\n", strings.Join(strings.Fields(err.Error()), " "))
			return
		}
		summary := fmt.Sprintf("%d ms", elapsed)
		if len(details) > 0 {
			summary += ", " + strings.Join(details, ", ")
		}
		fmt.Fprintf(writer, "SUCCESS! (%s)\n", summary)
	}
}

// Go's logger always appends a newline; leave the action line open for stop.
type actionWriter struct {
	io.Writer
}

func (writer actionWriter) Write(message []byte) (int, error) {
	length := len(message)
	if length == 0 || message[length-1] != '\n' {
		return writer.Writer.Write(message)
	}
	written, err := writer.Writer.Write(message[:length-1])
	if err == nil && written == length-1 {
		return length, nil
	}
	return written, err
}
