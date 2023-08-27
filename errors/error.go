package errors

import (
	stderr "errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// Error is an error type with embedded stack trace and optional logging
// attributes.
type Error struct {
	stack      Stack
	msg        string
	underlying error
	fields     map[string]any
}

// New returns a new error value with embedded location and stack trace.
func New(text string) *Error {
	return &Error{
		stack: getStack(),
		msg:   text,
	}
}

// Errorf returns a new formatted error value with embedded location and stack
// trace.
func Errorf(msg string, v ...any) *Error {
	err := fmt.Errorf(msg, v...)
	return &Error{
		stack:      getStack(),
		msg:        err.Error(),
		underlying: stderr.Unwrap(err),
	}
}

// Error conforms to the stdlib error interface, allowing you to return this as
// an error value.`
func (e *Error) Error() string {
	return e.msg
}

// Unwrap allows this error to be unwrapped such that errors embedded using
// [Errorf] with the `%w` print verb can be extracted.
func (e *Error) Unwrap() error {
	return e.underlying
}

// With adds a field and a value to this error that will be logged alongside it
// in any message using the srv package. This is a fluent interface and can
// be called in a chain to add multiple values
func (e *Error) With(field string, value any) *Error {
	if e.fields == nil {
		e.fields = map[string]any{}
	}
	e.fields[field] = value
	return e
}

// Location returns the source location where the error was created.
func (e *Error) Location() *Location {
	return e.stack[0]
}

// Stack returns the callstack for the given error.
func (e *Error) Stack() Stack {
	return e.stack
}

// Fields returns a []any of any fields added to the error using [Error.With]
// with each key prefixed with `err_` to set them apart from other logged
// attributes.
func (e *Error) Fields() []any {
	if len(e.fields) == 0 {
		return nil
	}
	fieldList := make([]any, 0, len(e.fields)/2)
	for k, v := range e.fields {
		fieldList = append(fieldList, "err_"+k, v)
	}
	return fieldList
}

// Stack is the call stack from the location of the error up to main.main()
type Stack []*Location

// String renders the call stack in the format of:
//
//	<function>(<file>:<line>) ► [...]
func (s Stack) String() string {
	var sb strings.Builder
	for i := len(s) - 1; i >= 0; i-- {
		sb.WriteString(s[i].Function + `(` + s[i].File + `:` + strconv.Itoa(s[i].Line) + `)`)
		if i > 0 {
			sb.WriteString(` ► `)
		}
	}
	return sb.String()
}

// Location represents a single frame in the call stack.
type Location struct {
	File     string
	Function string
	Line     int
}

// String prints a Location in the form of `<file>:<line>`
func (l *Location) String() string {
	return l.File + `:` + strconv.Itoa(l.Line)
}

// Format implements [fmt.Formatter], allowing a Location to be printed using
// the following print verbs:
//
//   - %s, %v gives `<file>:<line>`
//   - %q     gives `"<file>:<line>"`
//   - %+v    gives `<file>:<line>(<function>)`
//   - %#v    gives `errors.Location{File: "<file>", Function: "<function>", Line: <line>}`
func (l *Location) Format(state fmt.State, verb rune) {
	switch verb {
	case 'q', 's':
		if verb == 'q' {
			state.Write([]byte(`"` + l.String() + `"`))
			return
		}
		state.Write([]byte(l.String()))
	case 'v':
		switch {
		case state.Flag('+'):
			state.Write([]byte(l.String() + `(` + l.Function + `)`))
		case state.Flag('#'):
			fmt.Fprintf(state, `errors.Location{File: %q, Function: %q, Line: %d}`, l.File, l.Function, l.Line)
		default:
			state.Write([]byte(l.String()))
		}
	default:
		state.Write([]byte(`%!` + string(verb) + `(errors.Location=` + l.String() + `)`))
	}
}

func getStack() Stack {
	stackptrs := make([]uintptr, 50)
	stackptrs = stackptrs[:runtime.Callers(3, stackptrs)]
	stack := make(Stack, 0, len(stackptrs))
	frames := runtime.CallersFrames(stackptrs)
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.File, "runtime/") {
			frame.File = filepath.Base(frame.File)
			if strings.HasPrefix(frame.Function, `main.`) && frame.Function[5] != '(' {
				frame.Function = strings.TrimPrefix(frame.Function, `main.`)
			}
			stack = append(stack, &Location{
				File:     frame.File,
				Function: frame.Function,
				Line:     frame.Line,
			})
		}

		if !more {
			break
		}
	}
	slices.Clip(stack)
	return stack
}
