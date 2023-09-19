package instrumentation

import (
	"context"
	"encoding/json"
	stderr "errors"
	"log/slog"
	"sync/atomic"

	"andy.dev/srv/errors"
	"andy.dev/srv/internal/logfmt"
	"github.com/go-kit/kit/metrics"
)

// TODO: allow setting log level with an atomic
type HandlerOptions struct {
	Name           string
	MinLevel       slog.Level
	OverrideParent bool
	ShowLocation   bool
	TrimCode       bool
	ErrorCounter   metrics.Counter
	WarnCounter    metrics.Counter
	InfoCounter    metrics.Counter
}

type Handler struct {
	name string
	// location       log.CodeLocation
	formatter      slog.Handler
	parent         *Handler
	leveler        *slog.LevelVar
	overrideParent *atomic.Bool
	doCode         bool
	trimCode       bool
	metrics        *handlerMetrics
	errorCounter   metrics.Counter
	warnCounter    metrics.Counter
	infoCounter    metrics.Counter
}

type handlerMetrics struct{}

func NewHandler(h slog.Handler, options HandlerOptions) *Handler {
	leveler := &slog.LevelVar{}
	leveler.Set(options.MinLevel)

	newHandler := &Handler{
		name:           options.Name,
		formatter:      h,
		leveler:        leveler,
		overrideParent: &atomic.Bool{},
		doCode:         options.ShowLocation,
		trimCode:       options.TrimCode,
		errorCounter:   options.ErrorCounter,
		warnCounter:    options.WarnCounter,
		infoCounter:    options.InfoCounter,
	}

	if options.OverrideParent {
		newHandler.overrideParent.Store(true)
	}

	// if we are layering on top of another SrvHandler (like for a sublogger),
	// we should keep its formetter, so that we don't build a Handle chain, but also retain a reference to it so that
	// we do not log below its level.
	if sh, ok := h.(*Handler); ok {
		newHandler.formatter = sh.formatter
		newHandler.parent = sh
	}
	return newHandler
}

// Enabled tells a logger whether we have been configured to log at a given
// level or not. It is free to ignore this, but both [slog.Logger] and the srv
// logger type will honor this. If we have a parent set, we check in with them
// (and down the parent chain) to see if they allow logging at this level too.
// This way logging can be controlled from a root level.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	// Potential optimization: move from parent->child on change instead.
	if h.parent == nil {
		return level >= h.leveler.Level()
	}
	return level >= h.leveler.Level() && (h.parent.Enabled(context.Background(), level) || h.overrideParent.Load())
}

// Handle implements slog.Handler. This is where errors are instrumented and
// counted and where the decision to print source location is made.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// increment the appropriate level counter, if attached
	var counter metrics.Counter
	switch r.Level {
	case slog.LevelError:
		counter = h.errorCounter
	case slog.LevelWarn:
		counter = h.warnCounter
	case slog.LevelInfo:
		counter = h.infoCounter
	}
	if counter != nil {
		counter.Add(1)
	}
	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if err, isErr := a.Value.Any().(error); isErr {
			if a.Key != "err" {
				// TODO: reconsider munging with user-provided keys
				a.Key = "err_" + a.Key
			}
			nr.AddAttrs(a)
			var sErr *errors.Error
			if stderr.As(err, &sErr) {
				errData := make([]any, 0, 2+len(sErr.Fields()))
				errData = append(errData, "location", sErr.Location().String())
				errData = append(errData, sErr.Fields()...)
				if h.leveler.Level() <= slog.LevelDebug {
					errData = append(errData, "stacktrace", sErr.Stack().String())
				}
				nr.AddAttrs(slog.Group(a.Key+"_data", errData...))
			}
			return true
		}
		nr.AddAttrs(a)
		return true
	})
	if h.doCode {
		loc := logfmt.FmtRecord(nr, h.trimCode)
		if loc != "" {
			nr.AddAttrs(slog.String(slog.SourceKey, loc))
		}
	}
	return h.formatter.Handle(ctx, nr)
}

// WithAttrs is necessary to implement [slog.Handler], and, since this is a
// wrapper it simply calls the underlying handler's method and returns a shallow
// copy of itself.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := h.clone()
	nh.formatter = h.formatter.WithAttrs(attrs)
	return nh
}

// WithGroup is necessary to implement [slog.Handlerl. As with WithGroup, it
// returns a clone.
func (h *Handler) WithGroup(name string) slog.Handler {
	nh := h.clone()
	nh.formatter = h.formatter.WithGroup(name)
	return nh
}

func (h *Handler) Name() string {
	return h.name
}

func (h *Handler) SetLevel(newLevel slog.Level, overrideParent bool) bool {
	changed := false
	if h.leveler.Level() != newLevel {
		changed = true
		h.leveler.Set(newLevel)
	}
	if h.overrideParent.Load() != overrideParent {
		changed = true
		h.overrideParent.Store(overrideParent)
	}
	return changed
}

func (h *Handler) MarshalJSON() ([]byte, error) {
	type handlerJSON struct {
		Name     string `json:"name,omitempty"`
		Level    string `json:"level"`
		Override bool   `json:"override_parent,omitempty"`
	}
	return json.Marshal(handlerJSON{
		Name:     h.name,
		Level:    h.leveler.Level().String(),
		Override: h.overrideParent.Load(),
	})
}

func (h *Handler) GetLevel() (currentLevel slog.Level, override bool) {
	return h.leveler.Level(), h.overrideParent.Load()
}

func (h *Handler) clone() *Handler {
	clone := *h
	return &clone
}
