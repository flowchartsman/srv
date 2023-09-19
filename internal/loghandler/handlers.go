package loghandler

import (
	"io"
	"log/slog"
	"os"

	"andy.dev/srv/internal/loghandler/human"
	"github.com/mattn/go-isatty"
)

func NewJSON(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	})
}

func NewText(w io.Writer) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	})
}

func NewAuto(w io.Writer) slog.Handler {
	if isTerm(w) {
		return human.NewHandler(human.HandlerOpts{
			MinLevel: slog.LevelDebug,
			DoSource: false,
			// We assume human logging doesn't need to see the service they are
			// currently running.
			IgnoreAttrs: []string{"service"},
		}, w)
	}
	return NewJSON(w)
}

func isTerm(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		if isatty.IsTerminal(f.Fd()) {
			return true
		}
	}
	return false
}
