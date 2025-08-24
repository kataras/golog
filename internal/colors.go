package internal

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/kataras/golog/internal/terminal"
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

// Rich returns a formatted string with color and style codes.
// If colors are disabled, returns the plain text.
func Rich(text string, colorCode int, options ...RichOption) string {
	if !terminal.SupportColors || !terminal.IsTerminal(os.Stdout) {
		return text
	}

	var codes []int
	codes = append(codes, colorCode)

	for _, opt := range options {
		switch opt {
		case Background:
			codes = append(codes, colorCode+10) // Background colors are +10
		case Underline:
			codes = append(codes, 4)
		case Bold:
			codes = append(codes, 1)
		}
	}

	if len(codes) == 1 {
		return fmt.Sprintf("\033[%dm%s\033[0m", codes[0], text)
	}

	// Multiple codes
	codeStr := fmt.Sprintf("%d", codes[0])
	for _, code := range codes[1:] {
		codeStr += fmt.Sprintf(";%d", code)
	}

	return fmt.Sprintf("\033[%sm%s\033[0m", codeStr, text)
}

// WriteRich writes a formatted string with color and style to the writer.
func WriteRich(w io.Writer, text string, colorCode int, options ...RichOption) (int, error) {
	// If it's a printer, check each writer's support for rich text.
	if p, ok := w.(*Printer); ok {
		var lastErr error
		var n int

		var (
			richData  []byte
			plainData = []byte(text)
		)

		for i, writer := range p.writers {
			var (
				written int
				err     error
			)

			supportsColor := p.rich[i]
			if supportsColor {
				if richData == nil { // set once.
					richData = []byte(Rich(text, colorCode, options...))
				}
				written, err = writer.Write(richData)
			} else {
				written, err = writer.Write(plainData)
			}

			if err != nil {
				lastErr = err
			}
			if written > n {
				n = written
			}
		}

		return n, lastErr
	}

	if SupportsColor(w) {
		richText := Rich(text, colorCode, options...)
		return w.Write([]byte(richText))
	}

	return w.Write([]byte(text))
}

// SupportsColor determines if the output supports ANSI color codes.
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

// NopOutput returns a writer that discards all writes (equivalent to pio.NopOutput).
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
