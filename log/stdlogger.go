package log

import (
	"log/slog"
	"regexp"
)

// StdLogLeveler is a function to determine the log message to assign messages
// from a [*stdlog.Logger].
type StdLogLeveler func(logMsg []byte) (newMsg []byte, level slog.Level)

// StdLogStatic returns a StdLogLeveler that will log all messages at the
// provided level.
func StdLogStatic(logLevel slog.Level) StdLogLeveler {
	return func(msg []byte) ([]byte, slog.Level) {
		return msg, logLevel
	}
}

var levelRx = regexp.MustCompile(`(?i)^\[?(info?|warn(?:ing)?|err(?:or)?|d(?:bg|ebug))(?:\]|\s?:|\s?-)\s*`)

// StdLogGuess returns a StdLogLeveler that will attempt to guess the log level
// of messages coming from a [*stdlog.Logger] using a regular expression that
// looks for lines beginning with the following patterns:
//
//		LVL:
//		LVL-
//		[LVL]
//
//	 Where LVL is one of:
//			dbg, debug, inf, info, warn, warning, err, error
func StdLogGuess(msg []byte) ([]byte, slog.Level) {
	if len(msg) > 0 {
		// TODO: this can be statemachine-ified to make it more performant.
		levelIdx := levelRx.FindSubmatchIndex(msg)
		if len(levelIdx) >= 4 {
			var foundLevel slog.Level
			switch msg[levelIdx[2]] {
			case 'D', 'd':
				foundLevel = slog.LevelDebug
			case 'I', 'i':
				foundLevel = slog.LevelInfo
			case 'W', 'w':
				foundLevel = slog.LevelWarn
			case 'E', 'e':
				foundLevel = slog.LevelError
			}
			return msg[levelIdx[1]:], foundLevel
		}

	}
	return msg, slog.LevelInfo
}

// stdLogWrapper is a type implementing [io.Writer], and is used by
// [*Logger.StdLogger] to generate [*log.Logger] values for those dependencies
// which consumer them.
type stdLogWrapper struct {
	logger  *Logger
	leveler StdLogLeveler
}

// Write implements io.Writer for stdlog redirection.
func (w *stdLogWrapper) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := len(p)
	if p[n-1] == '\n' {
		// Trim CR added by stdlog.
		p = p[0 : n-1]
	}
	msg, level := w.leveler(p)
	w.logger.Log(defaultCtx, level, Up(3), string(msg))
	return n, nil
}
