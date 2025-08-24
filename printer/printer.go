package printer

import (
	"bufio"
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
	// map of writer index to whether it supports [rich text[0m
	rich map[int]bool // no struct, cus we need to delete the records when adding new writers.
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
func (p *Printer) WriteString(s string) (n int, err error) {
	if s == "" {
		return 0, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var data []byte
	for _, w := range p.writers {
		if sw, ok := w.(io.StringWriter); ok {
			n, err = sw.WriteString(s)
		} else {
			if data == nil {
				data = []byte(s)
			}
			n, err = w.Write(data)
		}
		if err != nil {
			return
		}
		if n != len(s) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(s), nil
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
	if len(a) == 0 {
		return p.Write([]byte(format))
	}

	return fmt.Fprintf(p, format, a...)
}

// toString converts any to string, handling common types.
func toString(v any) string {
	if v == nil {
		return "<nil>"
	}

	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

// Scan scans from the provided reader and writes lines to the printer.
// It returns a cancel function to stop the scanning operation.
func (p *Printer) Scan(r io.Reader) (cancel func()) {
	scanner := bufio.NewScanner(r)
	stop := make(chan struct{})

	go func() {
		defer close(stop)
		for scanner.Scan() {
			select {
			case <-stop:
				return
			default:
				line := scanner.Bytes()
				if len(line) > 0 {
					// Write the line with a newline
					data := make([]byte, len(line)+1)
					copy(data, line)
					data[len(line)] = '\n'
					p.Write(data)
				}
			}
		}
	}()

	return func() {
		select {
		case <-stop:
		default:
			close(stop)
		}
	}
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
