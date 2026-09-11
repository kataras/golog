package terminal

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// TestIsTerminalNonFile covers the branch every platform's implementation
// shares: anything that is not an *os.File cannot be a terminal, and asking
// must not panic or touch a file descriptor.
func TestIsTerminalNonFile(t *testing.T) {
	writers := map[string]io.Writer{
		"bytes.Buffer": &bytes.Buffer{},
		"io.Discard":   io.Discard,
		"nil":          nil,
	}

	for name, w := range writers {
		if IsTerminal(w) {
			t.Errorf("IsTerminal(%s) = true, want false", name)
		}
	}
}

// TestIsTerminalRealFile checks the *os.File path against a file that is
// definitely not a terminal. It cannot assert the true case: `go test` runs
// with its output redirected, so there is no terminal to point at.
func TestIsTerminalRealFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "golog-terminal-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	t.Cleanup(func() { f.Close() })

	if IsTerminal(f) {
		t.Error("IsTerminal(regular file) = true, want false")
	}
}

// TestIsTerminalDoesNotPanicOnStdHandles is the smoke test for the syscall
// itself. The answer depends on how the test was invoked, so only the fact that
// it returns is asserted.
func TestIsTerminalDoesNotPanicOnStdHandles(t *testing.T) {
	for _, f := range []*os.File{os.Stdout, os.Stderr, os.Stdin} {
		_ = IsTerminal(f)
	}
}

func TestSupportColorsIsSettable(t *testing.T) {
	prev := SupportColors
	t.Cleanup(func() { SupportColors = prev })

	SupportColors = false
	if SupportColors {
		t.Error("SupportColors should be writable by callers that want colors off")
	}
}
