package printer

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kataras/golog/printer/terminal"
)

// forceColors makes richText emit escape codes regardless of what the host
// terminal can do, so the encoding can be asserted on any platform and in CI.
func forceColors(t *testing.T) {
	t.Helper()

	prev := terminal.SupportColors
	terminal.SupportColors = true
	t.Cleanup(func() { terminal.SupportColors = prev })
}

func TestRichText(t *testing.T) {
	forceColors(t)

	tests := []struct {
		name    string
		color   int
		options []RichOption
		want    string
	}{
		{name: "color only", color: Red, want: "\033[31mtext\033[0m"},
		{name: "background", color: Red, options: []RichOption{Background}, want: "\033[31;41mtext\033[0m"},
		{name: "underline", color: Cyan, options: []RichOption{Underline}, want: "\033[36;4mtext\033[0m"},
		{name: "bold", color: Green, options: []RichOption{Bold}, want: "\033[32;1mtext\033[0m"},
		{
			name:    "several styles keep their order",
			color:   Yellow,
			options: []RichOption{Bold, Underline},
			want:    "\033[33;1;4mtext\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RichText("text", tt.color, tt.options...); got != tt.want {
				t.Errorf("RichText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRichTextRespectsSupportColors(t *testing.T) {
	prev := terminal.SupportColors
	terminal.SupportColors = false
	t.Cleanup(func() { terminal.SupportColors = prev })

	if got := RichText("text", Red); got != "text" {
		t.Errorf("RichText() = %q, want the plain text when SupportColors is off", got)
	}
}

// TestWriteRichUsesTheWritersOwnAnswer is the regression test for the bug where
// Rich re-asked about os.Stdout after the caller had already decided about the
// actual destination. A Printer whose writer supports color must emit color
// even when this process's stdout is a pipe, which is exactly what it is under
// `go test`.
func TestWriteRichUsesTheWritersOwnAnswer(t *testing.T) {
	forceColors(t)

	p := NewPrinter(io.Discard)
	// Claim color support for the writer, the way setRich would for a terminal.
	p.rich[0] = true

	var buf bytes.Buffer
	p.writers[0] = &buf

	if _, err := p.WriteRich("[INFO]", Cyan); err != nil {
		t.Fatalf("WriteRich: %v", err)
	}

	if got, want := buf.String(), "\033[36m[INFO]\033[0m"; got != want {
		t.Errorf("WriteRich wrote %q, want %q", got, want)
	}
}

func TestWriteRichPlainForNonColorWriters(t *testing.T) {
	forceColors(t)

	var color, plain bytes.Buffer
	p := NewPrinter(&color)
	p.AddOutput(&plain)
	p.rich[0], p.rich[1] = true, false

	if _, err := p.WriteRich("[WARN]", Magenta); err != nil {
		t.Fatalf("WriteRich: %v", err)
	}

	if got, want := color.String(), "\033[35m[WARN]\033[0m"; got != want {
		t.Errorf("color writer got %q, want %q", got, want)
	}
	if got, want := plain.String(), "[WARN]"; got != want {
		t.Errorf("plain writer got %q, want %q", got, want)
	}
}

func TestPrinterWriteFanOut(t *testing.T) {
	var a, b bytes.Buffer

	p := NewPrinter(&a)
	p.AddOutput(&b)

	if _, err := p.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := p.WriteString(" world"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}

	for name, buf := range map[string]*bytes.Buffer{"first": &a, "second": &b} {
		if got, want := buf.String(), "hello world"; got != want {
			t.Errorf("%s writer = %q, want %q", name, got, want)
		}
	}
}

func TestPrinterEmptyWritesAreNoOps(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf)

	if n, err := p.Write(nil); n != 0 || err != nil {
		t.Errorf("Write(nil) = %d, %v; want 0, nil", n, err)
	}
	if n, err := p.WriteString(""); n != 0 || err != nil {
		t.Errorf(`WriteString("") = %d, %v; want 0, nil`, n, err)
	}
	if buf.Len() != 0 {
		t.Errorf("an empty write reached the output: %q", buf.String())
	}
}

func TestPrinterSetOutputReplaces(t *testing.T) {
	var first, second bytes.Buffer

	p := NewPrinter(&first)
	p.SetOutput(&second)
	p.Write([]byte("only to the second"))

	if first.Len() != 0 {
		t.Errorf("SetOutput did not replace the first writer, it got %q", first.String())
	}
	if got, want := second.String(), "only to the second"; got != want {
		t.Errorf("second writer = %q, want %q", got, want)
	}
}

func TestPrinterPrintHelpers(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf)

	p.Print("a")
	p.Print([]byte("b"))
	p.Print(struct{}{}) // any other type renders as fmt.Sprint would.
	p.Print(nil)
	p.Println("c")
	p.Printf("%s-%d", "d", 1)
	p.Printf("literal")

	if got, want := buf.String(), "ab{}<nil>c\nd-1literal"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestNopOutput(t *testing.T) {
	w := NopOutput()

	if !IsNop(w) {
		t.Error("IsNop should report true for NopOutput")
	}
	if IsNop(io.Discard) {
		t.Error("IsNop should report false for io.Discard")
	}
	if SupportsColor(w) {
		t.Error("a nop writer cannot support colors")
	}
	if SupportsColor(nil) {
		t.Error("a nil writer cannot support colors")
	}

	if n, err := w.Write([]byte("dropped")); n != 7 || err != nil {
		t.Errorf("nop Write = %d, %v; want 7, nil", n, err)
	}
	if n, err := w.(io.StringWriter).WriteString("dropped"); n != 7 || err != nil {
		t.Errorf("nop WriteString = %d, %v; want 7, nil", n, err)
	}
}

func TestPrinterTerminalFallbacks(t *testing.T) {
	// A bytes.Buffer is never a terminal, so Terminal reports none.
	p := NewPrinter(&bytes.Buffer{})

	if _, ok := p.Terminal(); ok {
		t.Error("Terminal() should report false when no writer is a terminal")
	}
	if got := p.TerminalOrStdout(); got == nil {
		t.Error("TerminalOrStdout() must not return nil")
	}
	if got := p.TerminalOrStderr(); got == nil {
		t.Error("TerminalOrStderr() must not return nil")
	}
}

func TestPrinterClone(t *testing.T) {
	var original bytes.Buffer

	p := NewPrinter(&original)
	c := p.Clone()

	var added bytes.Buffer
	c.AddOutput(&added)

	c.Write([]byte("to the clone"))
	if added.Len() == 0 {
		t.Error("the clone's added writer received nothing")
	}

	original.Reset()
	p.Write([]byte("to the original"))
	if added.Len() != len("to the clone") {
		t.Error("a writer added to the clone leaked into the original")
	}
}

func TestPrinterScan(t *testing.T) {
	var buf lockedBuffer

	p := NewPrinter(&buf)
	pr, pw := io.Pipe()
	cancel := p.Scan(pr)

	io.WriteString(pw, "first\nsecond\n")
	pw.Close()

	waitFor(t, func() bool { return strings.Contains(buf.String(), "second") })

	if got := buf.String(); got != "first\nsecond\n" {
		t.Errorf("scanned output = %q, want %q", got, "first\nsecond\n")
	}

	// Same double-close bug as golog.Logger.Scan: the goroutine used to close
	// the stop channel too, so a cancel that landed after the reader ended
	// panicked.
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(cancel)
	}
	wg.Wait()
	cancel()
}

// TestConcurrentWriteRichAndAddOutput is the race detector's target: WriteRich
// used to read writers and rich without holding the lock that AddOutput and
// SetOutput take to replace them.
func TestConcurrentWriteRichAndAddOutput(t *testing.T) {
	p := NewPrinter(io.Discard)

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 200 {
				p.WriteRich("[INFO]", Cyan)
				p.Write([]byte(" line\n"))
				p.WriteString("string\n")
			}
		})
	}
	wg.Go(func() {
		for range 200 {
			p.AddOutput(io.Discard)
		}
	})
	wg.Go(func() {
		for range 200 {
			p.SetOutput(io.Discard)
		}
	})
	wg.Go(func() {
		for range 200 {
			p.Clone()
		}
	})

	wg.Wait()
}

// lockedBuffer is a bytes.Buffer safe to read while the scan goroutine writes.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	for range 500 {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met in time")
}
