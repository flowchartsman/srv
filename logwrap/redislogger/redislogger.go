package redislogger

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
)

// New returns a logger that will conform to the redis client's logging interface
func New(from *slog.Logger) *Logger {
	return &Logger{from}
}

// Logger satisfies redis' logging interface
type Logger struct {
	l *slog.Logger
}

var em = regexp.MustCompile(`(?i)error|failed`)

// Printf prints a standard printf log message at either Info or Failed log
// levels, depending on the redis message
func (rl *Logger) Printf(ctx context.Context, format string, v ...any) {
	if em.MatchString(format) {
		rl.l.ErrorContext(ctx, fmt.Sprintf(format, v...))
	} else {
		rl.l.InfoContext(ctx, fmt.Sprintf(format, v...))
	}
}
