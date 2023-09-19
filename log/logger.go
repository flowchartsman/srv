package log

import (
	"context"
	"fmt"
	stdlog "log"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// Logger is a logging type with several extra convenience methods, including
// <METHOD>f variants for formatted output.
//
// Non-formatted methods take a list of attrs as key-value pairs, with one
// exception. If the first argument is an error value, it will be treated
// specially, and automatically given the "err" key.
//
// Under the hood, this wraps a [*slog.Logger], which can be retrieved with the
// [Slogger] method for passing to dependencies that support it.
type Logger struct {
	s *slog.Logger
}

var defaultCtx = context.Background()

// NewLogger returns a new logger wrapping an [*slog.Logger]
func NewLogger(slogger *slog.Logger) *Logger {
	return &Logger{slogger}
}

// Debug logs at LevelDebug.
func (l *Logger) Debug(msg string, attrs ...any) {
	l.Log(defaultCtx, slog.LevelDebug, Up(1), msg, attrs...)
}

// Debugf logs a formatted message at LevelDebug.
func (l *Logger) Debugf(format string, args ...any) {
	l.Logf(defaultCtx, slog.LevelDebug, Up(1), format, args...)
}

// Info logs at LevelInfo.
func (l *Logger) Info(msg string, attrs ...any) {
	l.Log(defaultCtx, slog.LevelInfo, Up(1), msg, attrs...)
}

// Infof logs a formatted message at LevelInfo.
func (l *Logger) Infof(format string, args ...any) {
	l.Logf(defaultCtx, slog.LevelInfo, Up(1), format, args...)
}

// Warn logs at LevelWarn.
func (l *Logger) Warn(msg string, attrs ...any) {
	l.Log(defaultCtx, slog.LevelWarn, Up(1), msg, attrs...)
}

// Warnf logs a formatted message at LevelWarn.
func (l *Logger) Warnf(format string, args ...any) {
	l.Logf(defaultCtx, slog.LevelWarn, Up(1), format, args...)
}

// Error logs at LevelError.
func (l *Logger) Error(msg string, attrs ...any) {
	l.Log(defaultCtx, slog.LevelError, Up(1), msg, attrs...)
}

// Errorf logs a formatted message at LevelError.
func (l *Logger) Errorf(format string, args ...any) {
	l.Logf(defaultCtx, slog.LevelError, Up(1), format, args...)
}

// Trace tracks the duration of a function or span of code. It returns a
// closure, that, when called, will log the message along with a duration at the
// Info level. It can be particularly useful for use with defer to track the
// execution time of the current functon.
//
//	// Example
//	trace := log.Trace()
//	result := someFunction()
//	trace()
//
//	// Example with defer
//	func (t *MyType) SomeMethod(arg string) error {
//	    defer t.Logger.Trace()()
//	}
func (l *Logger) Trace(msg string, attrs ...any) TraceFn {
	if !l.Enabled(slog.LevelInfo) {
		return func() {}
	}
	started := time.Now()
	caller := Up(1)
	return func() {
		l.Log(defaultCtx, slog.LevelInfo, caller, msg, append(attrs, "duration", time.Since(started))...)
	}
}

// TraceDebug is exactly like Trace, only it will log traces at the debug level.
func (l *Logger) TraceDebug(msg string, attrs ...any) TraceFn {
	if !l.Enabled(slog.LevelDebug) {
		return func() {}
	}
	started := time.Now()
	caller := Up(1)
	return func() {
		l.Log(defaultCtx, slog.LevelDebug, caller, msg, append(attrs, "duration", time.Since(started))...)
	}
}

// TraceThreshold works like TraceDebuge, except that it will log durations
// which exceed the threshold duration at the warning level and ordinary
// durations at the debug level. If the thresholdPercent value is >0, it will
// only warn if the duration exceeds the threshold by a certain percent.
//
// As with the other tracing methods, log messages will have the `duration`
// attribute appended, representing the duration of the span. If
// thresholdPercent is >0, `duration_percent` will also be added.
func (l *Logger) TraceThreshold(threshold time.Duration, thresholdPercent float64, msg string, attrs ...any) TraceFn {
	if !l.Enabled(slog.LevelWarn) {
		return func() {}
	}
	started := time.Now()
	caller := Up(1)
	return func() {
		var newAttrs []any
		lvl := slog.LevelDebug
		duration := time.Since(started)
		if thresholdPercent > 0 {
			newAttrs = make([]any, len(attrs), len(attrs)+2)
			copy(newAttrs, attrs)
			pctDiff := float64(duration-threshold) / float64(threshold)
			newAttrs = append(newAttrs, "duration_percent", pctDiff)
			if pctDiff > thresholdPercent {
				lvl = slog.LevelWarn
			}
		} else {
			newAttrs = make([]any, len(attrs), len(attrs)+1)
			copy(newAttrs, attrs)
			if duration > threshold {
				lvl = slog.LevelWarn
			}
		}
		newAttrs = append(newAttrs, "duration", duration)
		l.Log(defaultCtx, lvl, caller, msg, newAttrs...)
	}
}

// TraceErr tracks the duration of a function call by returning a function that,
// when called with an error value, will log the message along with a duration
// at either the Info or Error levels, depending on whether the error is nil.
//
// Example:
//
//	errTrace := log.TraceErr()
//	result, err := someFunction()
//	trace(err)
func (l *Logger) TraceErr(msg string, attrs ...any) TraceErrFn {
	started := time.Now()
	caller := Up(1)
	return func(err error) {
		if err != nil {
			l.Log(defaultCtx, slog.LevelError, caller, msg, append(attrs, "duration", time.Since(started), "err", err)...)
			return
		}
		l.Log(defaultCtx, slog.LevelInfo, caller, msg, append(attrs, "duration", time.Since(started))...)
	}
}

// Enabled reports whether l emits log records at the given context and level.
func (l *Logger) Enabled(level slog.Level) bool {
	return l.s.Enabled(defaultCtx, level)
}

// With returns a Logger that includes the given attributes in each output
// operation. Arguments are converted to attributes as if by [Logger.Log].
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		l.s.With(args...),
	}
}

// WithGroup returns a Logger that starts a group, if name is non-empty. The
// keys of all attributes added to the Logger will be qualified by the given
// name. (How that qualification happens depends on the [Handler.WithGroup]
// method of the Logger's Handler.)
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		l.s.WithGroup(name),
	}
}

// Slogger returns the underlying [*slog.Logger]
func (l *Logger) Slogger() *slog.Logger {
	return l.s
}

// StdLogger returns a [*stdlog.Logger] suitable for passing to dependencies
// which consume the standard log type.
func (l *Logger) StdLogger(leveler StdLogLeveler) *stdlog.Logger {
	return stdlog.New(&stdLogWrapper{
		logger:  l,
		leveler: leveler,
	}, "", 0)
}

// Log logs a message at the specified level, at the specified code location.
// This is a low-level logging method intended for wrapping and precise control
// of the logging location in cases such as wrapping or helper calls.
func (l *Logger) Log(ctx context.Context, lvl slog.Level, location CodeLocation, message string, attrs ...any) {
	if !l.Enabled(lvl) {
		return
	}
	r := slog.NewRecord(time.Now(), lvl, message, uintptr(location))
	if len(attrs) > 0 && attrIsErr(attrs[0]) {
		r.Add(slog.Attr{
			Key:   "err",
			Value: slog.AnyValue(attrs[0]),
		})
		attrs = attrs[1:]
	}
	r.Add(attrs...)
	l.s.Handler().Handle(ctx, r)
}

// Logf logs a formatted message at the specified level, at the specified code
// location. This is a low-level logging method intended for wrapping and
// precise control of the logging location in cases such as wrapping or helper
// calls.
func (l *Logger) Logf(ctx context.Context, lvl slog.Level, location CodeLocation, format string, args ...any) {
	if !l.Enabled(lvl) {
		return
	}
	r := slog.NewRecord(time.Now(), lvl, fmt.Sprintf(format, args...), uintptr(location))
	l.s.Handler().Handle(ctx, r)
}

// CodeLocation represents the source of a logging line.
type CodeLocation uintptr

func (cl CodeLocation) String() string {
	fs := runtime.CallersFrames([]uintptr{uintptr(cl)})
	f, _ := fs.Next()
	if f.Line > 0 {
		f.File = filepath.Base(f.File)
		return f.File + `:` + strconv.Itoa(f.Line)
	}
	return ""
}

const (
	// NoLocation can be used in conjunction with [Logger.Log] and [Logger.Logf]
	// to specify that the log should have no location.
	NoLocation CodeLocation = 0
)

// Up returns the location of the caller at skip levels above the current
// location.
func Up(skip int) CodeLocation {
	var pcs [1]uintptr
	runtime.Callers(skip+2, pcs[:])
	return CodeLocation(pcs[0])
}

func attrIsErr(v any) bool {
	_, ok := v.(error)
	return ok
}

// TraceFn is the closure returned from [Logger.Trace] for tracing
// execution time of a process or function.
type TraceFn func()

// TraceErrFn is the closure returned from [Logger.TraceErr] for tracing
// execution time of a process that returns an [error] value.
type TraceErrFn func(error)
