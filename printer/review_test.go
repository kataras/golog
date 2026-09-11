package printer

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kataras/golog/printer/terminal"
)

func TestPrinterPrintfEscapesPercent(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf)
	p.Printf("100%% done")
	if got, want := buf.String(), "100% done"; got != want {
		t.Errorf("Printf = %q, want %q", got, want)
	}
}

func TestPrinterPrintAnyValue(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf)
	p.Print(42)
	p.Println(errors.New("e"))
	if got, want := buf.String(), "42e\n"; got != want {
		t.Errorf("Print/Println = %q, want %q", got, want)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("nope") }

func TestPrinterWriteStringContinuesPastFailure(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(failingWriter{})
	p.AddOutput(&buf)

	n, err := p.WriteString("abc")
	if err == nil {
		t.Error("WriteString swallowed the writer's error")
	}
	if n != 3 || buf.String() != "abc" {
		t.Errorf("WriteString stopped at the failing writer: n=%d, second writer got %q", n, buf.String())
	}
}

func TestAppendRichMatchesRichText(t *testing.T) {
	prev := terminal.SupportColors
	t.Cleanup(func() { terminal.SupportColors = prev })
	terminal.SupportColors = true

	got := string(AppendRich([]byte("pre:"), "[INFO]", Cyan, Bold))
	if want := "pre:" + RichText("[INFO]", Cyan, Bold); got != want {
		t.Errorf("AppendRich = %q, want %q", got, want)
	}

	terminal.SupportColors = false
	if got := string(AppendRich(nil, "[INFO]", Cyan)); got != "[INFO]" {
		t.Errorf("AppendRich with colors off = %q, want plain", got)
	}
}

// countingWriter records how many Write calls it received.
type countingWriter struct {
	buf    bytes.Buffer
	writes int
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++
	return w.buf.Write(p)
}

func TestWriteRichLineIsOneWritePerWriter(t *testing.T) {
	prev := terminal.SupportColors
	t.Cleanup(func() { terminal.SupportColors = prev })
	terminal.SupportColors = true

	var plain, richW countingWriter
	p := NewPrinter(&plain)
	p.AddOutput(&richW)
	p.rich[1] = true // pretend the second writer is a terminal.

	if _, err := p.WriteRichLine("[INFO]", Cyan, nil, []byte(" hello\n")); err != nil {
		t.Fatal(err)
	}

	if plain.writes != 1 || richW.writes != 1 {
		t.Errorf("writes = %d and %d, want exactly one each", plain.writes, richW.writes)
	}
	if got, want := plain.buf.String(), "[INFO] hello\n"; got != want {
		t.Errorf("plain writer got %q, want %q", got, want)
	}
	if got, want := richW.buf.String(), RichText("[INFO]", Cyan)+" hello\n"; got != want {
		t.Errorf("rich writer got %q, want %q", got, want)
	}
}

func TestScanLinesTruncatesAndReportsErrors(t *testing.T) {
	var lines []string
	var errs []error
	stop := make(chan struct{})

	huge := strings.Repeat("z", MaxScanLine+10)
	r := io.MultiReader(strings.NewReader("one\r\n\n"+huge+"\nlast"), &failAfter{err: errors.New("boom")})
	ScanLines(r, stop, func(b []byte) { lines = append(lines, string(b)) }, func(err error) { errs = append(errs, err) })

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %q", len(lines), lines)
	}
	if lines[0] != "one" || lines[2] != "last" {
		t.Errorf("lines = %q", []string{lines[0], lines[2]})
	}
	if got := lines[1]; len(got) != MaxScanLine+len(ScanTruncated) || !strings.HasSuffix(got, ScanTruncated) {
		t.Errorf("huge line: len %d, suffix %q", len(got), got[max(0, len(got)-20):])
	}
	if len(errs) != 1 || errs[0].Error() != "boom" {
		t.Errorf("errs = %v, want [boom]", errs)
	}
}

// failAfter is a reader that fails immediately.
type failAfter struct{ err error }

func (f *failAfter) Read([]byte) (int, error) { return 0, f.err }
