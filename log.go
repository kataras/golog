package golog

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"uuid"
)

// A Log represents a log line.
type Log struct {
	// Logger is the original printer of this Log.
	Logger *Logger `json:"-"`
	// ID uniquely identifies this log entry.
	//
	// It is the zero UUID, and omitted from the output, unless the Logger has
	// a NewID generator set. See `Logger.SetIDGenerator`.
	ID uuid.UUID `json:"id,omitzero"`
	// Time is the time of fired.
	Time time.Time `json:"-"`
	// Timestamp is the unix time in second of fired.
	Timestamp int64 `json:"timestamp,omitzero"`
	// Level is the log level.
	Level Level `json:"level"`
	// Message is the string reprensetation of the log's main body.
	Message string `json:"message"`
	// Fields any data information useful to represent this log.
	Fields Fields `json:"fields,omitempty"`
	// Stacktrace contains the stack callers when on `Debug` level.
	// The first one should be the Logger's direct caller function.
	Stacktrace []Frame `json:"stacktrace,omitempty"`
	// NewLine returns false if this Log
	// derives from a `Print` function,
	// otherwise true if derives from a `Println`, `Error`, `Errorf`, `Warn`, etc...
	//
	// This NewLine does not mean that `Message` ends with "\n" (or `pio#NewLine`).
	// NewLine has to do with the methods called,
	// not the original content of the `Message`.
	NewLine bool `json:"-"`

	// timeFormat is the Logger's TimeFormat at the moment the entry was made,
	// so FormatTime does not have to read the Logger while it may be changing.
	timeFormat string
	// buf is the scratch space the plain-text line is built in. It lives on
	// the pooled Log so that a line costs no allocation of its own.
	buf []byte
}

// Field returns the value stored in this Log's Fields under "key" as a T.
//
// It reports false when the key is absent or its value is not a T:
//
//	username, ok := log.Field[string]("username")
//
// Shorthand for `log.Fields.Get`, which is where the lookup actually happens.
func (l *Log) Field[T any](key string) (T, bool) {
	return l.Fields.Get[T](key)
}

// Frame represents the log's caller.
type Frame struct {
	// Function is the package path-qualified function name of
	// this call frame. If non-empty, this string uniquely
	// identifies a single function in the program.
	// This may be the empty string if not known.
	Function string `json:"function"`
	// Source contains the file name and line number of the
	// location in this frame. For non-leaf frames, this will be
	// the location of a call.
	Source string `json:"source"`
}

// String method returns the concat value of "file:line".
// Implements the `fmt.Stringer` interface.
func (f Frame) String() string {
	return f.Source
}

// FormatTime returns the formatted `Time`.
func (l *Log) FormatTime() string {
	if l.timeFormat == "" {
		return ""
	}
	return l.Time.Format(l.timeFormat)
}

var funcNameReplacer = strings.NewReplacer(")", "", "(", "", "*", "")

// GetStacktrace tries to return the callers of this function.
func GetStacktrace(limit int) (callerFrames []Frame) {
	return appendStacktrace(nil, limit)
}

// appendStacktrace is GetStacktrace into a caller-owned slice, so that a
// pooled Log can reuse the one it already has.
func appendStacktrace(dst []Frame, limit int) []Frame {
	if limit < 0 {
		return nil
	}
	dst = dst[:0]

	var pcs [32]uintptr
	n := runtime.Callers(1, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	for {
		f, more := frames.Next()

		if f.File != "" && !skipFrame(f) { // keep it here, break should be respected.
			dst = append(dst, Frame{
				Function: shortFunctionName(f.Function),
				Source:   f.File + ":" + strconv.Itoa(f.Line),
			})

			if limit > 0 && len(dst) >= limit {
				break
			}
		}

		if !more {
			break
		}
	}

	return dst
}

// skipFrame reports whether a frame is the standard library's or golog's own,
// neither of which is the caller anyone wants to see.
func skipFrame(f runtime.Frame) bool {
	if isStdlibFrame(f.Function) {
		return true
	}

	if rest, ok := strings.CutPrefix(f.Function, "github.com/kataras/golog"); ok &&
		(rest == "" || rest[0] == '.' || rest[0] == '/') {
		// golog's own frames, matched on the function's package path rather
		// than on the file path, which does not contain the module path when
		// golog is built from a checkout. The examples, the tests and the
		// integrations are the exceptions: there golog is the caller.
		file := filepath.ToSlash(f.File)
		return !strings.Contains(file, "/_examples/") &&
			!strings.HasSuffix(file, "_test.go") &&
			!strings.HasSuffix(file, "/integration.go")
	}

	return false
}

// isStdlibFrame reports whether "function" (as runtime.Frame.Function spells
// it) belongs to the standard library.
//
// A standard library package path has no dot in its first element
// ("runtime.Callers", "net/http.(*conn).serve"), a module path does
// ("github.com/kataras/iris/v12.(*Application).Listen"). The old check looked
// for "go/src/" in the file path instead, which is also where GOPATH-layout
// projects live, so those got empty stacktraces. Packages "main" and
// "command-line-arguments" are the caller's, whatever their path.
func isStdlibFrame(function string) bool {
	head, _, hasSlash := strings.Cut(function, "/")
	if hasSlash {
		return !strings.Contains(head, ".")
	}

	pkg, _, _ := strings.Cut(head, ".")
	return pkg != "main" && pkg != "command-line-arguments"
}

// shortFunctionName trims a fully qualified function name down to what a
// person reads: "Application.Listen" for a method, "context/Do" for a
// package-level function.
func shortFunctionName(funcName string) string {
	if before, after, ok := strings.Cut(funcName, ".("); ok && len(before) > 1 {
		// e.g. method: github.com/kataras/iris/v12.(*Application).Listen to:
		//      Application.Listen
		return funcNameReplacer.Replace(after)
	}

	if _, after, ok := strings.CutLast(funcName, "/"); ok && after != "" {
		// e.g. package-level function: github.com/kataras/iris/v12/context.Do to
		// context/Do
		return strings.Replace(after, ".", "/", 1)
	}

	return funcName
}
