package printer

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"sync"

	"github.com/kataras/golog/printer/terminal"
)

// Printer is a simple printer that manages multiple output writers
// and provides thread-safe atomic writes.
type Printer struct {
	mu      sync.Mutex
	writers []io.Writer
	// rich maps a writer's index to whether it renders ANSI color. Rebuilt by
	// setRich whenever the writer set changes.
	rich map[int]bool
	// Scratch buffers for WriteRichLine, reused under mu.
	plainLine, richLine []byte
}

// NewPrinter creates a new Printer with the given initial writer.
func NewPrinter(writer io.Writer) *Printer {
	return &Printer{
		writers: []io.Writer{writer},
		rich:    map[int]bool{0: SupportsColor(writer)},
	}
}

func (p *Printer) setRich() {
	if p.rich == nil {
		p.rich = make(map[int]bool, len(p.writers))
	}

	for i, w := range p.writers {
		value := SupportsColor(w)
		p.rich[i] = value
	}
}

// SetOutput replaces all current writers with the single provided writer.
func (p *Printer) SetOutput(w io.Writer) {
	p.mu.Lock()
	p.writers = []io.Writer{w}
	p.setRich()
	p.mu.Unlock()
}

// AddOutput adds one or more writers to the printer.
func (p *Printer) AddOutput(writers ...io.Writer) {
	p.mu.Lock()
	p.writers = append(p.writers, writers...)
	p.setRich()
	p.mu.Unlock()
}

// Terminal returns a new Printer that includes the writers that output destination is a terminal kind.
// If no terminal writers exist, it returns nil and false.
func (p *Printer) Terminal() (*Printer, bool) {
	var terminalWriters []io.Writer
	p.mu.Lock()
	for _, w := range p.writers {
		if terminal.IsTerminal(w) {
			terminalWriters = append(terminalWriters, w)
		}
	}
	p.mu.Unlock()
	if len(terminalWriters) == 0 {
		return nil, false
	}

	newPrinter := &Printer{
		writers: terminalWriters,
	}
	newPrinter.setRich()
	return newPrinter, true
}

// TerminalOrStdout returns an io.Writer that includes the writers that output destination is a terminal kind.
// If no terminal writers exist, it returns os.Stdout.
func (p *Printer) TerminalOrStdout() io.Writer {
	t, ok := p.Terminal()
	if !ok {
		return os.Stdout
	}
	return t
}

// TerminalOrStderr returns an io.Writer that includes the writers that output destination is a terminal kind.
// If no terminal writers exist, it returns os.Stderr.
func (p *Printer) TerminalOrStderr() io.Writer {
	t, ok := p.Terminal()
	if !ok {
		return os.Stderr
	}
	return t
}

// WriteRich writes a formatted string with color and style to all registered writers.
// It checks each writer's support for rich text and writes accordingly.
func (p *Printer) WriteRich(text string, colorCode int, options ...RichOption) (int, error) {
	return p.WriteRichLine(text, colorCode, options, nil)
}

// maxPooledLineBuffer is the largest scratch buffer WriteRichLine keeps alive
// between calls.
const maxPooledLineBuffer = 64 << 10

// WriteRichLine writes "title", colored for the writers that render color and
// plain for the ones that do not, immediately followed by "body", as exactly
// one Write per writer.
//
// This is how a log line reaches the writers whole: the title is the only part
// that differs per writer, so the two renderings are built once each and each
// writer receives its line in a single call. Anything less lets another
// goroutine writing to the same file slip in between the pieces.
func (p *Printer) WriteRichLine(title string, colorCode int, style []RichOption, body []byte) (int, error) {
	// Held for the same reason Write holds it: writers and rich are replaced
	// wholesale by SetOutput and grown by AddOutput, so reading them without
	// the lock races with either.
	p.mu.Lock()
	defer p.mu.Unlock()

	p.plainLine = append(append(p.plainLine[:0], title...), body...)

	var (
		rich    []byte // built on first use.
		lastErr error
		n       int
	)

	for i, writer := range p.writers {
		data := p.plainLine
		if p.rich[i] && title != "" {
			if rich == nil {
				// AppendRich, not Rich: this writer has already been asked
				// whether it supports color (see setRich). Rich would answer
				// again for os.Stdout instead, and strip the color here
				// whenever this process's stdout is redirected.
				p.richLine = AppendRich(p.richLine[:0], title, colorCode, style...)
				p.richLine = append(p.richLine, body...)
				rich = p.richLine
			}
			data = rich
		}

		written, err := writer.Write(data)
		if err != nil {
			lastErr = err
		}
		if written > n {
			n = written
		}
	}

	if cap(p.plainLine) > maxPooledLineBuffer {
		p.plainLine = nil
	}
	if cap(p.richLine) > maxPooledLineBuffer {
		p.richLine = nil
	}

	return n, lastErr
}

// Write writes data to all registered writers atomically.
func (p *Printer) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	var n int

	for _, w := range p.writers {
		written, err := w.Write(data)
		if err != nil {
			lastErr = err
		}
		if written > n {
			n = written
		}
	}

	return n, lastErr
}

// WriteString writes a string to all registered writers atomically.
//
// Like Write, it tries every writer even after one of them fails, and returns
// the last error and the largest count written.
func (p *Printer) WriteString(s string) (int, error) {
	if s == "" {
		return 0, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var (
		data    []byte
		lastErr error
		n       int
	)

	for _, w := range p.writers {
		var (
			written int
			err     error
		)
		if sw, ok := w.(io.StringWriter); ok {
			written, err = sw.WriteString(s)
		} else {
			if data == nil {
				data = []byte(s)
			}
			written, err = w.Write(data)
		}

		if err == nil && written != len(s) {
			err = io.ErrShortWrite
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

// Print writes the string representation of v to all writers.
func (p *Printer) Print(v any) (int, error) {
	data := []byte(toString(v))
	return p.Write(data)
}

// Println writes the string representation of v followed by a newline to all writers.
func (p *Printer) Println(v any) (int, error) {
	data := []byte(toString(v) + "\n")
	return p.Write(data)
}

// Printf writes a formatted string to all writers.
func (p *Printer) Printf(format string, a ...any) (int, error) {
	// Always through Fprintf, even with no arguments: "%%" has to become "%".
	return fmt.Fprintf(p, format, a...)
}

// toString converts any to string, handling common types.
func toString(v any) string {
	switch s := v.(type) {
	case nil:
		return "<nil>"
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprint(v)
	}
}

// MaxScanLine caps one logical line read by ScanLines. Past it the rest of the
// line is dropped and ScanTruncated is appended to what was kept, so a child
// process that writes megabytes without a newline cannot grow memory without
// bound, and a reader can tell a clipped record from a complete one.
const MaxScanLine = 1 << 20

// ScanTruncated marks a line that hit MaxScanLine.
const ScanTruncated = " [truncated]"

// ScanLines reads "r" line by line and calls "emit" for every complete,
// non-empty line, without its trailing newline (and without a carriage return
// before it, for CRLF input). It returns when "r" is exhausted, when "stop" is
// closed, or on a read error; an error other than io.EOF is passed to "onError"
// unless "stop" has been closed, in which case it is the expected consequence
// of the caller shutting the reader down.
//
// Lines longer than MaxScanLine are truncated as described there. The slice
// passed to "emit" is reused for the next line, so it must not be retained.
//
// Logger.Scan and Printer.Scan are both built on it.
func ScanLines(r io.Reader, stop <-chan struct{}, emit func(line []byte), onError func(error)) {
	br := bufio.NewReader(r)

	var (
		line     []byte
		dropping bool // the current line passed MaxScanLine.
	)

	for {
		select {
		case <-stop:
			return
		default:
		}

		frag, isPrefix, err := br.ReadLine()
		if len(frag) > 0 && !dropping {
			if room := MaxScanLine - len(line); len(frag) > room {
				line = append(line, frag[:room]...)
				line = append(line, ScanTruncated...)
				dropping = true
			} else {
				line = append(line, frag...)
			}
		}

		if err == nil && isPrefix {
			continue // more of the same line to come.
		}

		// ReadLine never returns data together with an error, so an error
		// means the last line, if any, has already been emitted.
		if err == nil {
			trimmed := bytes.TrimRight(line, "\r")
			if len(trimmed) > 0 {
				emit(trimmed)
			}
			line = line[:0]
			dropping = false
			continue
		}

		if err != io.EOF && onError != nil {
			select {
			case <-stop:
			default:
				onError(err)
			}
		}
		return
	}
}

// Scan scans from the provided reader and writes lines to the printer.
// It returns a cancel function to stop the scanning operation, which is safe
// to call more than once. A read error other than the end of the input is
// written to the printer as "scan: <error>".
func (p *Printer) Scan(r io.Reader) (cancel func()) {
	stop := make(chan struct{})

	go ScanLines(r, stop,
		func(line []byte) { _, _ = p.Write(append(line, '\n')) },
		func(err error) { _, _ = fmt.Fprintf(p, "scan: %v\n", err) },
	)

	return sync.OnceFunc(func() { close(stop) })
}

// Clone creates a deep copy of the Printer, including its writers and rich map.
func (p *Printer) Clone() *Printer {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Create a new Printer with the same writers and rich map
	newRich := make(map[int]bool, len(p.rich))
	maps.Copy(newRich, p.rich)

	newWriters := make([]io.Writer, 0, len(p.writers)) // Deep copy of writers slice.
	for _, w := range p.writers {
		if clonable, ok := w.(interface{ Clone() io.Writer }); ok {
			newWriters = append(newWriters, clonable.Clone())
		} else {
			newWriters = append(newWriters, w)
		}
	}

	return &Printer{
		writers: newWriters,
		rich:    newRich,
	}
}
