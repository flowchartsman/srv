package human

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/fatih/color"

	"andy.dev/srv/internal/logfmt"
)

type cPrinter func(io.Writer, ...any)

var (
	// optimization: color.New(<color>).SetWriter(w) -> color.UnsetWriter(w)
	// can take care of quoting and streaming escapes and whatnog
	warnP  = color.New(color.FgYellow).FprintFunc()
	errorP = color.New(color.FgRed).FprintFunc()
	valP   = color.New(color.FgHiBlack).FprintFunc()
	msgP   = color.New(color.Bold).FprintFunc()
	keyP   = color.New(color.FgGreen).Add(color.Faint).FprintFunc()
)

type HandlerOpts struct {
	MinLevel    slog.Leveler
	DoSource    bool // TODO: should always print
	IgnoreAttrs []string
}

// Handler makes logs easy to read from the CLI
type Handler struct {
	mu        sync.Mutex
	minlevel  slog.Leveler
	writer    io.Writer
	doSource  bool
	ignore    map[string]bool
	logger    string
	group     string
	static    string
	staticErr string
}

func NewHandler(options HandlerOpts, w io.Writer) *Handler {
	ignore := make(map[string]bool, len(options.IgnoreAttrs))
	for _, ignored := range options.IgnoreAttrs {
		ignore[ignored] = true
	}
	return &Handler{
		minlevel: options.MinLevel,
		doSource: options.DoSource,
		ignore:   ignore,
		writer:   w,
	}
}

// doesn't log below its set level
func (s *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= s.minlevel.Level()
}

// implements Handler.Handle.
func (s *Handler) Handle(_ context.Context, r slog.Record) error {
	out := s.writer

	var errAttrs []slog.Attr
	var manSource string
	// var component string

	buf := GetBuf()
	defer PutBuf(buf)

	if s.group != "" {
		keyP(buf, s.group+"=")
		valP(buf, "[")
	}

	r.Attrs(func(a slog.Attr) bool {
		switch {
		case a.Key == slog.SourceKey:
			// manual source.
			manSource = a.Value.String()
		case a.Key == "logger":
			return true
		case strings.HasPrefix(a.Key, "err"):
			errAttrs = append(errAttrs, a)
		default:
			if s.ignore[a.Key] {
				return true
			}
			io.WriteString(buf, " ")
			printAttr(keyP, buf, a)
		}
		return true
	})

	// static ones at end
	buf.WriteString(s.static)

	if s.group != "" {
		valP(buf, " ]")
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
	if len(s.logger) > 0 {
		valP(out, "[")
		valP(out, s.logger)
		valP(out, "]")
	}
	valP(out, " - ")

	if r.Message == "" {
		if r.NumAttrs() == 0 && len(errAttrs) == 0 {
			msgP(out, "<no message>")
		}
	} else {
		msgP(out, r.Message)
	}

	// don't care about source line if it's not an error
	if r.Level > slog.LevelInfo {
		if manSource != "" { // user has manually specified a source location
			io.WriteString(out, " ")
			valP(out, "("+manSource+")")
		} else {
			if s.doSource {
				// don't trim, since srvhandler will make that call
				loc := logfmt.FmtRecord(r, false)
				if loc != "" {
					io.WriteString(out, " ")
					valP(out, "("+loc+")")
				}
			}
		}
	}

	if len(errAttrs) > 0 {
		for i := range errAttrs {
			io.WriteString(out, " ")
			printAttr(errorP, out, errAttrs[i])
		}
	}
	if s.staticErr != "" {
		io.WriteString(out, " ")
		io.WriteString(out, s.staticErr)
	}

	if buf.Len() > 0 {
		out.Write(buf.Bytes())
	}
	io.WriteString(out, "\n")
	return nil
}

// implement interface
func (s *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	attrs = slices.DeleteFunc(attrs, func(a slog.Attr) bool {
		return s.ignore[a.Key]
	})
	return s.clone(attrs...)
}

// implement interface
func (s *Handler) WithGroup(name string) slog.Handler {
	ns := s.clone()
	ns.group = name
	return ns
}

func (s *Handler) clone(newAttrs ...slog.Attr) *Handler {
	newHandler := &Handler{
		mu:        sync.Mutex{},
		writer:    s.writer,
		minlevel:  s.minlevel,
		doSource:  s.doSource,
		ignore:    s.ignore,
		group:     s.group,
		static:    s.static,
		staticErr: s.staticErr,
	}
	if len(newAttrs) == 0 && s.group == "" {
		return newHandler
	}
	abuf := GetBuf()
	defer PutBuf(abuf)
	errbuf := GetBuf()
	defer PutBuf(errbuf)
	if len(newAttrs) > 0 {
		for i := range newAttrs {
			switch {
			case newAttrs[i].Key == "source":
				continue
			case newAttrs[i].Key == "logger":
				if newHandler.logger != "" {
					newHandler.logger += "/" + newAttrs[i].Value.String()
				} else {
					newHandler.logger = newAttrs[i].Value.String()
				}
			case strings.HasPrefix(newAttrs[i].Key, "err"):
				errbuf.WriteString(" ")
				printAttr(errorP, errbuf, newAttrs[i])
			default:
				abuf.WriteString(" ")
				printAttr(keyP, abuf, newAttrs[i])
			}
		}
	}
	// if the old handler had a group, wrap all of its stuff in a group and put
	// it in static
	if newHandler.group != "" {
		abuf.WriteString(" ")
		keyP(abuf, newHandler.group+"=")
		valP(abuf, "[")
		if newHandler.staticErr != "" {
			abuf.WriteString(s.staticErr)
			abuf.WriteString(" ")
			newHandler.staticErr = ""
		}
		if newHandler.static != "" {
			abuf.WriteString(newHandler.static)
			abuf.WriteString(" ")
			newHandler.static = ""
		}
		valP(abuf, " ]")
	}
	// otherwise, just append the old errors and attrs onto the end of their
	// respective static sections
	if errbuf.Len() > 0 {
		if len(newHandler.staticErr) > 0 {
			errbuf.WriteString(" ")
			errbuf.WriteString(newHandler.staticErr)
		}
		newHandler.staticErr = errbuf.String()
	}
	if abuf.Len() > 0 {
		if len(newHandler.static) > 0 {
			abuf.WriteString(" ")
			abuf.WriteString(newHandler.static)
		}
		newHandler.static = abuf.String()
	}
	return newHandler
}

func printAttr(cp cPrinter, w io.Writer, a slog.Attr) {
	cp(w, qs(a.Key)+"=")
	r := a.Value.Resolve()
	switch r.Kind() {
	case slog.KindGroup:
		valP(w, "[ ")
		group := r.Group()
		for i, ga := range group {
			printAttr(keyP, w, ga)
			if i != len(group)-1 {
				io.WriteString(w, " ")
			}
		}
		valP(w, " ]")
	case slog.KindTime:
		valP(w, a.Value.Time().Format("Jan 02 15:04:05"))
	default:
		valP(w, qs(r.String()))
	}
}

func qs(s string) string {
	if needsQuoted(s) {
		return strconv.Quote(s)
	}
	return s
}

func needsQuoted(str string) bool {
	for len(str) > 0 {
		if str[0] < utf8.RuneSelf {
			switch str[0] {
			case '\n', '\t', ' ':
				return true
			}
			str = str[1:]
			continue
		}
		r, size := utf8.DecodeRuneInString(str)
		if unicode.IsSpace(r) {
			return true
		}
		str = str[size:]
	}
	return false
}
