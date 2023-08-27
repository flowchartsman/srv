package srv

import (
	"context"
	stderr "errors"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"

	"andy.dev/srv/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// TODO: route to allow setting log level with an atomic
type srvHandlerOptions struct {
	minlevel slog.Leveler
	doCode   bool
	trimCode bool
	errCt    prometheus.Counter
}

// srvhandler is the primary handler type for srv, and handles incrementing the
// master error count, code location logging, and advanced error annotation. It
// is intended to wrap another slog.Handler, which will handle log formatting.
type srvHandler struct {
	handler slog.Handler
	options *srvHandlerOptions
}

// newsrvHandler
func newsrvHandler(h slog.Handler, options srvHandlerOptions) *srvHandler {
	// Optimization: avoid stacking srvHandlers.
	if sh, ok := h.(*srvHandler); ok {
		return &srvHandler{sh.handler, &options}
	}
	return &srvHandler{h, &options}
}

// doesn't log below its set level
func (s *srvHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= s.options.minlevel.Level()
}

// Handle implements slog.Handler.
func (s *srvHandler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if err, isErr := a.Value.Any().(error); isErr {
			if a.Key != "err" {
				a.Key = "err_" + a.Key
			}
			nr.AddAttrs(a)
			var sErr *errors.Error
			if stderr.As(err, &sErr) {
				errData := make([]any, 0, 2+len(sErr.Fields()))
				errData = append(errData, "location", sErr.Location().String())
				errData = append(errData, sErr.Fields()...)
				if LogLevel(s.options.minlevel.Level()) <= LevelDebug {
					errData = append(errData, "stacktrace", sErr.Stack().String())
				}
				nr.AddAttrs(slog.Group(a.Key+"_data", errData...))
			}
			return true
		}
		nr.AddAttrs(a)
		return true
	})
	// increment the error counter if attached
	if nr.Level == slog.LevelError && s.options.errCt != nil {
		s.options.errCt.Add(1)
	}
	if s.options.doCode {
		loc := fmtLocation(nr, s.options.trimCode)
		if loc != "" {
			nr.AddAttrs(slog.String(slog.SourceKey, loc))
		}
	}
	return s.handler.Handle(ctx, nr)
}

// implement interface, and ensure options are shared if these somehow ever need to stack
func (s *srvHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &srvHandler{s.handler.WithAttrs(attrs), s.options}
}

// implement interface, and ensure options are shared if these somehow ever need to stack
func (s *srvHandler) WithGroup(name string) slog.Handler {
	return &srvHandler{s.handler.WithGroup(name), s.options}
}

func fmtLocation(r slog.Record, trim bool) string {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	if f.Line > 0 {
		if trim {
			f.File = filepath.Base(f.File)
		}
		return f.File + `:` + strconv.Itoa(f.Line)
	}
	return ""
}
