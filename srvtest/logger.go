package srvtest

import (
	"log/slog"
	"testing"

	"andy.dev/srv/log"
)

// ErrorStrategy defines what a [Logger] should do if a message is logged
// at the error level.
type ErrorStrategy int

const (
	// Ignore will take no action. This is the default.
	Ignore ErrorStrategy = iota
	// Fail will mark the test as failed, but continue.
	Fail
	// FailNow will mark the test as failed and exit immediately.
	FailNow
)

// Logger returns a [*log.Logger] that outputs all results using [testing/*t.Log].
func NewLogger(t *testing.T, options ...LoggerOption) *log.Logger {
	return log.NewLogger(slog.New(newHandler(t, options...)))
}
