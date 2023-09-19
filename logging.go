package srv

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"andy.dev/srv/internal/loghandler/instrumentation"
	"andy.dev/srv/log"
)

// Logger is an alias to the [log.Logger] type, so that you don't need to import
// the package to define your job functions
type Logger = log.Logger

// LogLevel represents a logging level. Aliased from [slog].
type LogLevel = slog.Level

// Logging Levels
const (
	LevelDebug = LogLevel(slog.LevelDebug)
	LevelInfo  = LogLevel(slog.LevelInfo)
	LevelWarn  = LogLevel(slog.LevelWarn)
	LevelError = LogLevel(slog.LevelError)
)

// Fatal logs a structured message at the error level with the root logger and
// exits the program immediately.
//
// NOTE: This bypasses any graceful shutdown handling,  so its use outside of
// main() is highly discouraged.
func (s *Srv) Fatal(msg string, attrs ...any) {
	s.fatal(log.Up(1), msg, attrs...)
}

// Fatal logs a formatted message at the error level with the root logger and
// exits the program immediately.
//
// NOTE: This bypasses any graceful shutdown handling,  so its use outside of
// main() is highly discouraged.
func (s *Srv) Fatalf(format string, args ...any) {
	s.fatal(log.Up(1), fmt.Sprintf(format, args...))
}

const (
	// ShowLocation will attach the filename and line number to the logging message
	LogLocation = 1 << iota
	// FullLocation will include the full path of the filename where the logging
	// message ocurred.
	LogFullLocation
	// NoMetrics will prevent the logger from tracking error, warn and info
	// message counts.
	NoMetrics
)

// NewLogger creates a [*log.Logger] that will attach a "logger" label to its
// output and metrics with the value of the provided name. If the consumer takes a [*slog.Logger], you can call the
// [Logger.Slogger] method to get it. Loggers will be tracked by srv,
// allowing for dynamic level modification at runtime via the `/loglevel route`,
// so multiple subloggers cannot have the same name.
func (s *Srv) NewLogger(name string, level LogLevel, flags int) *log.Logger {
	// check if the user has set a minimum log level that is less than the
	// minimum log level. If so, messages below this level won't
	// appear, so issue a warning about that.
	if !s.logger.Enabled(level) {
		rootLevel, _ := s.logHandler.GetLevel()
		s.warn(log.Up(2), "logger minimum level is less than current level", "logger", name, "current_level", rootLevel, "logger_level", level.String())
	}
	levelVar := slog.LevelVar{}
	levelVar.Set(level)
	handlerOpts := instrumentation.HandlerOptions{
		Name:         name,
		MinLevel:     level,
		ShowLocation: flags&LogLocation != 0,
		TrimCode:     flags&LogFullLocation == 0,
	}
	if flags&NoMetrics == 0 {
		handlerOpts.ErrorCounter = s.errCount.With("logger", name)
		handlerOpts.WarnCounter = s.warnCount.With("logger", name)
		handlerOpts.InfoCounter = s.infoCount.With("logger", name)
	}
	logHandler := instrumentation.NewHandler(s.logHandler, handlerOpts)
	s.loglevelHandler.AddLogHandler(logHandler)
	return log.NewLogger(slog.New(logHandler).With("logger", name))
}

// internal
// TODO: remove all <level>f messages for internal.
func (s *Srv) debugf(loc log.CodeLocation, format string, args ...any) {
	s.logger.Logf(context.Background(), slog.LevelDebug, loc, format, args...)
}

func (s *Srv) info(loc log.CodeLocation, msg string, attrs ...any) {
	s.logger.Log(context.Background(), slog.LevelInfo, loc, msg, attrs...)
}

func (s *Srv) warn(loc log.CodeLocation, msg string, attrs ...any) {
	s.logger.Log(context.Background(), slog.LevelWarn, loc, msg, attrs...)
}

func (s *Srv) warnf(loc log.CodeLocation, format string, args ...any) {
	s.logger.Logf(context.Background(), slog.LevelWarn, loc, format, args...)
}

func (s *Srv) error(loc log.CodeLocation, msg string, attrs ...any) {
	s.logger.Log(context.Background(), slog.LevelError, loc, msg, attrs...)
}

func (s *Srv) errorf(loc log.CodeLocation, format string, args ...any) {
	s.logger.Logf(context.Background(), slog.LevelError, loc, format, args...)
}

func (s *Srv) fatal(loc log.CodeLocation, msg string, attrs ...any) {
	s.termLogErr(loc, "SRV FATAL: "+msg, attrs...)
	s.closeTermlog()
	os.Exit(1)
}

func (s *Srv) termLogErr(loc log.CodeLocation, msg string, attrs ...any) {
	if s.logger == nil {
		basicLog(os.Stderr, loc, msg, attrs...)
	} else {
		s.error(loc, msg, attrs...)
	}
	s.termLog(loc, msg, attrs...)
}

func (s *Srv) termLog(loc log.CodeLocation, msg string, attrs ...any) {
	if s.termination == nil {
		return
	}
	basicLog(s.termination, loc, msg, attrs...)
}

func (s *Srv) closeTermlog() {
	if s.termination == nil {
		return
	}
	if err := s.termination.Close(); err != nil {
		if s.logger == nil {
			basicLog(os.Stderr, noloc, "failed to close termination log", err)
		} else {
			s.error(noloc, "failed to close termination log", err)
		}
	}
}

func basicLog(w io.Writer, loc log.CodeLocation, msg string, args ...any) {
	out := []byte(msg)
	out = fmt.Append(out, " ")
	if loc != noloc {
		args = append(args, "("+loc.String()+")")
	}
	out = fmt.Appendln(out, args...)
	w.Write(out)
}
