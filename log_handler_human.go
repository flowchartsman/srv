package srv

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/fatih/color"
)

type cPrinter func(io.Writer, ...any)

var (
	warnP  = color.New(color.FgYellow).FprintFunc()
	errorP = color.New(color.FgRed).FprintFunc()
	valP   = color.New(color.FgHiBlack).FprintFunc()
	msgP   = color.New(color.Bold).FprintFunc()
	keyP   = color.New(color.FgGreen).Add(color.Faint).FprintFunc()
)

type humanHandlerOpts struct {
	minlevel    slog.Leveler
	doSource    bool // TODO: should always print
	ignoreAttrs []string
}

// humanHandler makes logs easy to read from the CLI
type humanHandler struct {
	mu       sync.Mutex
	writer   io.Writer
	minlevel slog.Leveler
	doSource bool
	ignore   map[string]bool
	attrs    []slog.Attr
	group    string
}

func newHumanHandler(options humanHandlerOpts, w io.Writer) *humanHandler {
	ignore := make(map[string]bool, len(options.ignoreAttrs))
	for _, ignored := range options.ignoreAttrs {
		ignore[ignored] = true
	}
	return &humanHandler{
		minlevel: options.minlevel,
		doSource: options.doSource,
		ignore:   ignore,
		writer:   w,
	}
}

// doesn't log below its set level
func (s *humanHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= s.minlevel.Level()
}

// implements Handler.Handle.
func (s *humanHandler) Handle(_ context.Context, r slog.Record) error {
	out := s.writer

	var errAttrs []slog.Attr
	var manSource string
	// var component string

	last := GetBuf()
	defer PutBuf(last)
	if s.group != "" {
		keyP(last, s.group+"[ ")
	}

	r.Attrs(func(a slog.Attr) bool {
		switch {
		case a.Key == slog.SourceKey:
			// manual source.
			manSource = a.Value.String()
		case strings.HasPrefix(a.Key, "err"):
			errAttrs = append(errAttrs, a)
		// case a.Key == "component":
		// 	component = a.Value.String()
		default:
			if s.ignore[a.Key] {
				return true
			}
			printAttr(keyP, last, a)
			io.WriteString(last, " ")
		}
		return true
	})

	// static ones at end
	for i := range s.attrs {
		printAttr(keyP, last, s.attrs[i])
		io.WriteString(last, " ")
	}

	if s.group != "" {
		keyP(last, " ]")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	valP(out, r.Time.Format("Jan 02 15:04:05"))
	io.WriteString(out, " ")
	switch r.Level {
	case slog.LevelDebug:
		valP(out, "DBG")
	case slog.LevelInfo:
		io.WriteString(out, "INF")
	case slog.LevelWarn:
		warnP(out, "WRN")
	case slog.LevelError:
		errorP(out, "ERR")
	}
	valP(out, " - ")

	if r.Message == "" {
		if r.NumAttrs() == 0 && len(errAttrs) == 0 {
			msgP(out, "<no message>")
			io.WriteString(out, " ")
		}
	} else {
		msgP(out, r.Message)
		io.WriteString(out, " ")
	}

	if r.Level > slog.LevelInfo {
		if manSource != "" { // user has manually specified a source location
			valP(out, "("+manSource+")")
			io.WriteString(out, " ")
		} else {
			if s.doSource {
				// don't trim, since srvhandler will make that call
				loc := fmtLocation(r, false)
				if loc != "" {
					valP(out, "("+loc+")")
					io.WriteString(out, " ")
				}
			}
		}
	}

	if len(errAttrs) > 0 {
		for i, attr := range errAttrs {
			printAttr(errorP, out, attr)
			if i < len(errAttrs)-1 {
				io.WriteString(out, " ")
			}
		}
	}

	if last.Len() > 0 {
		out.Write(last.Bytes())
	}
	io.WriteString(out, "\n")
	return nil
}

// implement interface
func (s *humanHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	allowed := []slog.Attr{}
	for i := range attrs {
		if s.ignore[attrs[i].Key] {
			continue
		}
		allowed = append(allowed, attrs[i])
	}
	return s.clone(allowed...)
}

// implement interface
func (s *humanHandler) WithGroup(name string) slog.Handler {
	ns := s.clone()
	ns.group = name
	return ns
}

func (s *humanHandler) clone(newAttrs ...slog.Attr) *humanHandler {
	cAttrs := make([]slog.Attr, len(s.attrs), len(s.attrs)+len(newAttrs))
	copy(cAttrs, s.attrs)
	cAttrs = append(cAttrs, newAttrs...)
	return &humanHandler{
		mu:       sync.Mutex{},
		writer:   s.writer,
		minlevel: s.minlevel,
		doSource: s.doSource,
		ignore:   s.ignore,
		attrs:    cAttrs,
		group:    s.group,
	}
}

func printAttr(cp cPrinter, w io.Writer, a slog.Attr) {
	cp(w, a.Key)
	switch a.Value.Kind() {
	case slog.KindGroup:
		keyP(w, "[")
		group := a.Value.Group()
		for i, ga := range group {
			printAttr(keyP, w, ga)
			if i != len(group)-1 {
				io.WriteString(w, " ")
			}
		}
		keyP(w, "]")
	case slog.KindTime:
		keyP(w, "=")
		valP(w, a.Value.Time().Format("Jan 02 15:04:05"))
	default:
		keyP(w, "=")
		valP(w, a.Value.String())
	}
}
