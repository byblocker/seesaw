package utils

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

// Timer reports a sequential action on one line using the standard logger's writer.
// Its zero value is ready to use. Other output must not interrupt Start and Stop.
type Timer struct {
	started time.Time
	writer  io.Writer
}

func (timer *Timer) Start(action string) {
	if !timer.started.IsZero() {
		return
	}
	timer.writer = log.Writer()
	timer.started = time.Now()
	logger := log.New(actionWriter{timer.writer}, log.Prefix(), log.Flags())
	logger.Output(2, action+"... ")
}

// Go's logger always appends a newline; leave the action line open for Stop.
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

func (timer *Timer) Stop(err error, details ...string) {
	if timer.started.IsZero() {
		return
	}
	elapsed := time.Since(timer.started).Milliseconds()
	timer.started = time.Time{}
	if err != nil {
		fmt.Fprintf(timer.writer, "FAILED: %s\n", strings.Join(strings.Fields(err.Error()), " "))
		return
	}
	summary := fmt.Sprintf("%d ms", elapsed)
	if len(details) > 0 {
		summary += ", " + strings.Join(details, ", ")
	}
	fmt.Fprintf(timer.writer, "SUCCESS! (%s)\n", summary)
}
