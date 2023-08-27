package srv

import (
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

// defaultHandler is the log handler for the entire service. It handles
// things like attaching the service name to the metrics as well as
// attaching source references and incrementing the error counter if metrics
// are on.
var defaultH atomic.Pointer[srvHandler]

func defaultHandler() slog.Handler {
	return defaultH.Load()
}

func log(lvl slog.Level, location uintptr, message string, attrs ...any) {
	r := slog.NewRecord(time.Now(), lvl, message, location)
	r.Add(attrs...)
	defaultHandler().Handle(srvCtx, r)
}

func logf(lvl slog.Level, location uintptr, format string, args ...any) {
	r := slog.NewRecord(time.Now(), lvl, fmt.Sprintf(format, args...), location)
	defaultHandler().Handle(srvCtx, r)
}

// Debug logs messages at the debug level.
func Debug(msg string, attrs ...any) {
	log(slog.LevelDebug, location(1), msg, attrs...)
}

// Debugf logs formatted messages at the error level.
func Debugf(format string, args ...any) {
	logf(slog.LevelDebug, location(1), format, args...)
}

// Info logs messages at the info level.
func Info(msg string, attrs ...any) {
	log(slog.LevelInfo, location(1), msg, attrs...)
}

// Infof logs formatted messages at the error level.
func Infof(format string, args ...any) {
	logf(slog.LevelInfo, location(1), format, args...)
}

// Warn logs messages at the warn level.
func Warn(msg string, attrs ...any) {
	log(slog.LevelWarn, location(1), msg, attrs...)
}

// Warnf logs formatted messages at the error level.
func Warnf(format string, args ...any) {
	logf(slog.LevelWarn, location(1), format, args...)
}

// WarnV is a convenience method to log a message and an error value at the
// warning level.
func WarnV(msg string, err error, args ...any) {
	log(slog.LevelWarn, location(1), msg, append(args, "err", err)...)
}

// Error logs messages and optional error at the error level.
func Error(msg string, attrs ...any) {
	log(slog.LevelError, location(1), msg, attrs...)
}

// Errorf logs formatted messages at the error level.
func Errorf(format string, args ...any) {
	logf(slog.LevelError, location(1), format, args...)
}

// ErrorV is a convenience method to log a message and an error value at the
// error level.
func ErrorV(msg string, err error, args ...any) {
	log(slog.LevelError, location(1), msg, append(args, "err", err)...)
}

// Fatal logs a message at the error level and exits the program immediately
// without shutting down gracefully.
func Fatal(msg string, attrs ...any) {
	log(slog.LevelError, location(1), msg, attrs...)
	os.Exit(1)
}

// Fatalf logs a formatted message at the error level and exits the program
// immediately without shutting down gracefully.
func Fatalf(format string, args ...any) {
	logf(slog.LevelError, location(1), format, args...)
	os.Exit(1)
}

// FatalV is a convenience method to log a message and an error value at the
// error level and exit the program immediately.
func FatalV(msg string, err error, args ...any) {
	log(slog.LevelError, location(1), msg, append(args, "err", err)...)
	os.Exit(1)
}

// TODO: Leaving out Fail for now as use is confusing. Can add it back in if an
// explicit graceful fail is called for, but should the user ever request this
// or should they just die? Choosing the latter for now. -AW

// Fail logs a message at the error level and begins graceful shutdown. The
// calling goroutine will the block until the program exits.
// func Fail(msg string, err error, args ...any) {
// 	log(slog.LevelError, sourceLoc(1), msg, attrs...)
// 	srvShutdown()
// 	<-shutdownComplete
// 	os.Exit(1)
// }

// Failf logs a formatted message at the error level and begins graceful
// shutdown. The calling goroutine will the block until the program exits.
// func Failf(format string, formatArgs ...any) {
// 	logf(slog.LevelError, sourceLoc(1), format, args...)
// 	srvShutdown()
// 	<-shutdownComplete
// 	os.Exit(1)
// }

const noLocation uintptr = 0

// internal
func srvDebugf(loc uintptr, format string, args ...any) {
	logf(slog.LevelDebug, loc, format, args...)
}

func srvInfo(loc uintptr, message string, attrs ...any) {
	log(slog.LevelInfo, loc, message, attrs...)
}

func srvInfof(loc uintptr, format string, args ...any) {
	logf(slog.LevelInfo, loc, format, args...)
}

func srvWarnf(loc uintptr, format string, args ...any) {
	logf(slog.LevelWarn, loc, format, args...)
}

func srvErrorf(loc uintptr, format string, args ...any) {
	logf(slog.LevelError, loc, format, args...)
}

func srvErrorVal(loc uintptr, message string, err error) {
	log(slog.LevelError, loc, message, "err", err)
}

func srvFatalf(loc uintptr, format string, args ...any) {
	logf(slog.LevelError, loc, format, args...)
	os.Exit(1)
}

func srvFatalVal(loc uintptr, msg string, err error) {
	log(slog.LevelError, loc, msg, "err", err)
	os.Exit(1)
}
