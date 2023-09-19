package srvtest

// mostly copied from https://github.com/jba/slog/blob/main/handlers/simple/simple_handler.go

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/jba/slog/withsupport"
)

type LoggerOption func(*testHandler)

func WithErrorStrategy(es ErrorStrategy) LoggerOption {
	return func(th *testHandler) {
		th.es = es
	}
}

type testHandler struct {
	t   *testing.T
	es  ErrorStrategy
	goa *withsupport.GroupOrAttrs
}

func newHandler(t *testing.T, options ...LoggerOption) slog.Handler {
	th := &testHandler{
		t: t,
	}
	for _, o := range options {
		o(th)
	}
	return th
}

func (h *testHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return true
}

func (h *testHandler) WithGroup(name string) slog.Handler {
	h2 := *h
	h2.goa = h2.goa.WithGroup(name)
	return &h2
}

func (h *testHandler) WithAttrs(as []slog.Attr) slog.Handler {
	h2 := *h
	h2.goa = h2.goa.WithAttrs(as)
	return &h2
}

func (h *testHandler) Handle(ctx context.Context, r slog.Record) error {
	r2 := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	var attrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool { attrs = append(attrs, a); return true })
	for g := h.goa; g != nil; g = g.Next {
		if g.Group != "" {
			anys := make([]any, len(attrs))
			for i, a := range attrs {
				anys[i] = a
			}
			attrs = []slog.Attr{slog.Group(g.Group, anys...)}
		} else {
			attrs = append(slices.Clip(g.Attrs), attrs...)
		}
	}
	r2.AddAttrs(attrs...)
	h.tlog(r2)
	return nil
}

func (h *testHandler) tlog(r slog.Record) {
	msg := []byte{}
	fmt.Append(msg, r.Level.String()+":", r.Message)
	r.Attrs(func(a slog.Attr) bool {
		printattr(&msg, a)
		return true
	})
	h.t.Log(string(msg))
	switch h.es {
	case Fail:
		h.t.Fail()
	case FailNow:
		h.t.FailNow()
	}
}

func printattr(msg *[]byte, a slog.Attr) {
	rv := a.Value.Resolve()
	if rv.Kind() == slog.KindGroup {
		fmt.Append(*msg, qs(a.Key)+"[")
		for _, ga := range rv.Group() {
			printattr(msg, ga)
		}
		fmt.Append(*msg, "]")
		return
	}
	fmt.Append(*msg, qs(a.Key)+":", qs(a.Value.String()))
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
