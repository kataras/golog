package printer

import (
	"io"
	"os"
	"runtime"
	"strconv"

	"github.com/kataras/golog/printer/terminal"
)

// Standard color codes, any color code can be passed to `Rich` package-level function,
// when the destination terminal supports.
const (
	Black = 30 + iota
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
	Gray = White

	ColorReset = 0
)

// Style options for rich text formatting
type RichOption int

const (
	Background RichOption = iota
	Underline
	Bold
)

// Rich returns a formatted string with color and style codes when this
// process's os.Stdout is a terminal, and the plain text otherwise.
//
// It asks about os.Stdout on every call, because os.Stdout is an assignable
// variable that test helpers routinely swap. Nothing on golog's own write path
// calls Rich; the Printer decides per writer once, through SupportsColor.
func Rich(text string, colorCode int, options ...RichOption) string {
	if !terminal.IsTerminal(os.Stdout) {
		return text
	}

	return richText(text, colorCode, options...)
}

// RichText returns "text" wrapped in the ANSI color and style codes, without
// consulting os.Stdout the way Rich does.
//
// It is for callers that already know whether their destination can render
// colors -- because they were told (LevelMetadata.Text's enableColor) or
// because they asked about the writer itself (SupportsColor). Rich has to guess
// for them, and guesses about a different file descriptor.
//
// The process-wide terminal.SupportColors switch still applies.
func RichText(text string, colorCode int, options ...RichOption) string {
	return richText(text, colorCode, options...)
}

func richText(text string, colorCode int, options ...RichOption) string {
	if !terminal.SupportColors {
		return text
	}

	return string(AppendRich(make([]byte, 0, len(text)+16), text, colorCode, options...))
}

// AppendRich appends "text" wrapped in the ANSI color and style codes to
// "dst" and returns the extended slice. It is RichText for callers building a
// line in a buffer, which is what keeps a colored title from costing an
// allocation per log line.
//
// When terminal.SupportColors is off the plain text is appended instead.
func AppendRich(dst []byte, text string, colorCode int, options ...RichOption) []byte {
	if !terminal.SupportColors {
		return append(dst, text...)
	}

	// "\033[" + code(+";"+code)... + "m" + text + "\033[0m".
	dst = append(dst, '\033', '[')
	dst = strconv.AppendInt(dst, int64(colorCode), 10)

	for _, opt := range options {
		var code int
		switch opt {
		case Background:
			code = colorCode + 10 // background colors are +10.
		case Underline:
			code = 4
		case Bold:
			code = 1
		default:
			continue
		}

		dst = append(dst, ';')
		dst = strconv.AppendInt(dst, int64(code), 10)
	}

	dst = append(dst, 'm')
	dst = append(dst, text...)
	dst = append(dst, '\033', '[', '0', 'm')
	return dst
}

// WriteRich writes a formatted string with color and style to the writer.
func WriteRich(w io.Writer, text string, colorCode int, options ...RichOption) (int, error) {
	// If it's a printer, check each writer's support for rich text.
	if p, ok := w.(*Printer); ok {
		return p.WriteRich(text, colorCode, options...)
	}

	if SupportsColor(w) {
		return io.WriteString(w, richText(text, colorCode, options...))
	}

	return io.WriteString(w, text) // else plain text.
}

// SupportsColor determines if the output supports ANSI color codes.
//
// For an *os.File the answer is a syscall, so callers on a hot path should
// ask once and remember, as the Printer and Logger.SetLevelOutput do.
func SupportsColor(w io.Writer) bool {
	if w == nil {
		return false
	}

	isTerminal := !IsNop(w) && terminal.IsTerminal(w)
	if isTerminal && runtime.GOOS == "windows" {
		// if on windows then return true only when it does support 256-bit colors,
		// this is why we initially do that terminal check for the "w" writer.
		return terminal.SupportColors
	}

	return isTerminal
}

// NopOutput returns a writer that discards all writes.
func NopOutput() io.Writer {
	return &nopOutput{}
}

// IsNop can check wether an `w` io.Writer
// is a NopOutput.
func IsNop(w io.Writer) bool {
	if isN, ok := w.(interface {
		IsNop() bool
	}); ok {
		return isN.IsNop()
	}
	return false
}

type nopOutput struct{}

func (w *nopOutput) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *nopOutput) WriteString(s string) (int, error) {
	return len(s), nil
}

// IsNop defines this wrriter as a nop writer.
func (w *nopOutput) IsNop() bool {
	return true
}
