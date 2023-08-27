package stdlogger

import (
	"context"
	"log"
	"log/slog"
	"regexp"
	"runtime"
	"time"
)

// Wrapper is a provider for standard log logggers.
type Wrapper func(*slog.Logger) *log.Logger

// Leveler is a classifier for basic logs.
type Leveler interface {
	GetLevel(msg []byte) slog.Level
}

// StaticLevel returns a wrapper for a standard library logger that will log
// all messages at the specified level
func StaticLevel(logLevel slog.Level) Wrapper {
	return func(from *slog.Logger) *log.Logger {
		w := &stdlogWriter{from, staticLeveller(logLevel)}
		return log.New(w, "", 0)
	}
}

// DynamicLevel returns a wrapper for a standard library logger that will use
// the provided [Leveler] to determine log level.
func CustomLevel(leveler Leveler) Wrapper {
	return func(from *slog.Logger) *log.Logger {
		w := &stdlogWriter{from, leveler}
		return log.New(w, "", 0)
	}
}

// GuessLevel returns a wrapper for a standard library logger that will use
// basic heuristics to classify a log message. It looks for lines which begin
// with "error:", "[WARN]", "info -" etc. etc.
func GuessLevel() Wrapper {
	return func(from *slog.Logger) *log.Logger {
		w := &stdlogWriter{from, defaultLeveler{}}
		return log.New(w, "", 0)
	}
}

type stdlogWriter struct {
	logger  *slog.Logger
	leveler Leveler
}

// Write implements io.Writer for stdlog redirection.
func (w *stdlogWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := len(p)
	if p[n-1] == '\n' {
		// Trim CR added by stdlog.
		p = p[0 : n-1]
	}
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	r := slog.NewRecord(time.Now(), w.leveler.GetLevel(p), string(p), pcs[0])
	w.logger.Handler().Handle(context.Background(), r)
	return n, nil
}

type staticLeveller slog.Level

func (s staticLeveller) GetLevel([]byte) slog.Level {
	return slog.Level(s)
}

var levelRx = regexp.MustCompile(`(?i)^\[?(info?|warn(?:ing)?|err(?:or)?|d(?:bg|ebug))(?:\]|:|\s?-)`)

type defaultLeveler struct{}

func (defaultLeveler) GetLevel(msg []byte) slog.Level {
	if len(msg) > 0 {
		sms := levelRx.FindSubmatch(msg)
		if len(sms) > 1 {
			switch sms[1][0] {
			case 'D', 'd':
				return slog.LevelDebug
			case 'W', 'w':
				return slog.LevelWarn
			case 'E', 'e':
				return slog.LevelError
			}
		}
	}
	return slog.LevelInfo
}
