package srv

import (
	"context"
	"io"
	"log/slog"
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
)

type LogFormat int

const (
	formatBegin LogFormat = iota
	// FormatAuto will log using a human-readable format when attached to a
	// terminal, and JSON otherwise.
	FormatAuto
	// FormatJSON will log using JSON-formatted output.
	FormatJSON
	// FormatKV will log using a traditional field=value logging format.
	FormatKV
	// FormatHuman will log using a human-readable format
	FormatHuman
	// todo: FormatText
	// todo: FormatPrety
	formatEnd
)

// Alias slog types to avoid the import
type LogLevel int

// Log levels represent the lowest-allowable level of logging across the
// service. All log messages at the specified level and below will be logged,
// unless they are restricted further at the component level.
const (
	levelBegin = LogLevel(slog.LevelDebug - 1)
	// LevelDebug specifies debug level logging, which is the lowest. All
	// messages will be logged unless otherwise restricted by component.
	LevelDebug = LogLevel(slog.LevelDebug)
	// LevelInfo specifies info leve logging and above.
	LevelInfo = LogLevel(slog.LevelInfo)
	// LevelWarn specifies warn level logging and above.
	LevelWarn = LogLevel(slog.LevelWarn)
	// LevelError will log only error-level logging.
	LevelError = LogLevel(slog.LevelError)
	levelEnd   = LogLevel(slog.LevelError + 1)
)

func levelName(level LogLevel) string {
	switch level {
	case LevelDebug:
		return "LevelDebug"
	case LevelInfo:
		return "LevelInfo"
	case LevelWarn:
		return "LevelWarn"
	case LevelError:
		return "LevelError"
	default:
		return "UNKNOWN LOG LEVEL"
	}
}

type SubLoggerOptions int

const (
	_ SubLoggerOptions = iota
	// WithSource tells the sublogger to log the basename of the file and
	// line on which its message was logged from the dependency.
	WithSource
	// FullSource tells the sublogger to log the full file path and line
	// on which its message was logged from the dependency.
	FullSource
	// MeasureErrors tells the sublogger to increment the global error counter
	// for any messages logged at the Error level.
	MeasureErrors
)

// SubLogger returns an *slog.Logger with a static "component" field. It's
// useful for passing to dependencies that take a *slog.Logger.
func SubLogger(componentName string, minLevel LogLevel, options ...SubLoggerOptions) *slog.Logger {
	mustHaveInit("create a component logger")
	return newSubLogger(componentName, minLevel, options...)
}

// LogWrapper takes care of initializing a wrapped logger. Useful for quickly
// creating loggers for verious packages in srv/logwrap
func LogWrapper[T any](componentName string, wrapperFn func(*slog.Logger) T, minLevel LogLevel, options ...SubLoggerOptions) T {
	mustHaveInit("create a log wrapper")
	return wrapperFn(newSubLogger(componentName, minLevel, options...))
}

func newSubLogger(componentName string, minLevel LogLevel, options ...SubLoggerOptions) *slog.Logger {
	// check if the user has set a minimum log level that is less than the
	// minimum framework log level. If so, messages below this level won't
	// appear, so issue a warning about that.
	if !defaultHandler().Enabled(context.Background(), slog.Level(minLevel)) {
		srvWarnf(location(2), "sublogger %q has log level %s, which is less than the current minimum", componentName, levelName(minLevel))
	}
	doSource := false
	doTrim := true
	var counter prometheus.Counter
	for _, o := range options {
		switch o {
		case WithSource:
			doSource = true
		case FullSource:
			doSource = true
			doTrim = false
		case MeasureErrors:
			counter = errCount
		}
	}
	return slog.New(newsrvHandler(defaultHandler(), srvHandlerOptions{
		minlevel: slog.Level(minLevel),
		doCode:   doSource,
		trimCode: doTrim,
		errCt:    counter,
	}).WithAttrs([]slog.Attr{slog.String("component", componentName)}))
}

func newJSONHandler(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	})
}

func newKVHandler(w io.Writer) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	})
}

func newAutoHandler(w io.Writer) slog.Handler {
	if isTerm(w) {
		return newHumanHandler(humanHandlerOpts{
			minlevel:    slog.LevelDebug,
			doSource:    false,
			ignoreAttrs: []string{"service", "system"},
		}, w)
	}
	return newJSONHandler(w)
}

func location(skip int) uintptr {
	var pcs [1]uintptr
	runtime.Callers(skip+2, pcs[:])
	return pcs[0]
}
