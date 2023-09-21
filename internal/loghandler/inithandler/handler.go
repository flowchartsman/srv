package inithandler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Handler only logs very basic error messages for use when logging
// intialization fails or there's otherwise an issue with startup.
type Handler struct {
	w io.Writer
}

func New() *Handler {
	return &Handler{os.Stderr}
}

func (*Handler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl == slog.LevelError
}

// implements Handler.Handle.
func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	var errVal error
	r.Attrs(func(a slog.Attr) bool {
		switch {
		case a.Key == "err":
			errVal = a.Value.Any().(error)
		default:
			return true
		}
		return true
	})
	h.w.Write([]byte(r.Message))
	if errVal != nil {
		h.w.Write([]byte(fmt.Sprintf(" - %v", errVal)))
	}
	h.w.Write([]byte{'\n'})
	return nil
}

// WithAttrs implements slog.Handler
func (h *Handler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

// WithGroup implements slog.Handler
func (h *Handler) WithGroup(string) slog.Handler {
	return h
}
