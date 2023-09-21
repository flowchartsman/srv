package srv

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"

	"andy.dev/srv/internal/loghandler"
	"andy.dev/srv/internal/loghandler/instrumentation"
	"andy.dev/srv/internal/loglevelhandler"
	"andy.dev/srv/log"
)

// Default returns the default Logger.

const (
	noloc log.CodeLocation = 0
)

var (
	srvlogger       atomic.Value
	srvLogHandler   *instrumentation.Handler
	srvLevelHandler *loglevelhandler.Handler
)

func srvLogger() *log.Logger {
	return srvlogger.Load().(*log.Logger)
}

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

func initLogging(config *srvConfig) {
	var formatter slog.Handler
	switch config.logFormat {
	case "json":
		formatter = loghandler.NewJSON(os.Stderr)
	case "text":
		formatter = loghandler.NewText(os.Stderr)
	case "human":
		formatter = loghandler.NewHuman(os.Stderr)
	default:
		formatter = loghandler.NewAuto(os.Stderr)
	}
	var rootLevel slog.Level
	switch config.logLevel {
	case "debug":
		rootLevel = slog.LevelDebug
	case "info":
		// info is the default
	case "warn":
		rootLevel = slog.LevelWarn
	case "error":
		rootLevel = slog.LevelError
	}
	srvLogHandler = instrumentation.NewHandler(formatter, instrumentation.HandlerOptions{
		MinLevel:     rootLevel,
		ShowLocation: true,
		TrimCode:     true,
		ErrorCounter: srvErrors.With("logger", "root"),
		WarnCounter:  srvWarnings.With("logger", "root"),
		InfoCounter:  srvInfos.With("logger", "root"),
	})

	srvLevelHandler = loglevelhandler.NewHandler(srvLogHandler)
	srvlogger.Store(log.NewLogger(slog.New(srvLogHandler)))
}

// NewLogger creates a [*log.Logger] that will attach a "logger" label to its
// output and metrics with the value of the provided name. If the consumer takes a [*slog.Logger], you can call the
// [Logger.Slogger] method to get it. Loggers will be tracked by srv,
// allowing for dynamic level modification at runtime via the `/loglevel route`,
// so multiple subloggers cannot have the same name.
func NewLogger(name string, level LogLevel, flags int) *log.Logger {
	// check if the user has set a minimum log level that is less than the
	// minimum log level. If so, messages below this level won't
	// appear, so issue a warning about that.
	if !srvLogger().Enabled(level) {
		rootLevel, _ := srvLogHandler.GetLevel()
		sWarn(log.Up(2), "logger minimum level is less than current level", "logger", name, "current_level", rootLevel, "logger_level", level.String())
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
		handlerOpts.ErrorCounter = srvErrors.With("logger", name)
		handlerOpts.WarnCounter = srvWarnings.With("logger", name)
		handlerOpts.InfoCounter = srvInfos.With("logger", name)
	}
	logHandler := instrumentation.NewHandler(srvLogHandler, handlerOpts)
	srvLevelHandler.AddLogHandler(logHandler)
	return log.NewLogger(slog.New(logHandler).With("logger", name))
}

// internal
func sDebug(loc log.CodeLocation, msg string, attrs ...any) {
	srvLogger().Log(context.Background(), slog.LevelDebug, loc, msg, attrs...)
}

func sInfo(loc log.CodeLocation, msg string, attrs ...any) {
	srvLogger().Log(context.Background(), slog.LevelInfo, loc, msg, attrs...)
}

func sWarn(loc log.CodeLocation, msg string, attrs ...any) {
	srvLogger().Log(context.Background(), slog.LevelWarn, loc, msg, attrs...)
}

func sError(loc log.CodeLocation, msg string, attrs ...any) {
	srvLogger().Log(context.Background(), slog.LevelError, loc, msg, attrs...)
}

func sTermLogErr(loc log.CodeLocation, msg string, attrs ...any) {
	sError(loc, msg, attrs...)
	termlogWrite(loc, msg, attrs...)
}

func sFatal(loc log.CodeLocation, msg string, attrs ...any) {
	srvLogger().Log(context.Background(), slog.LevelError, loc, "FATAL: "+msg, attrs...)
	termlogWrite(loc, msg, attrs...)
	termlogClose()
	os.Exit(1)
}
