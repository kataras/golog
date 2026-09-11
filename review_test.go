package golog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"
)

// lockedBuffer is a bytes.Buffer that can be written by the logger and read by
// the test at the same time. A bare bytes.Buffer is not, and the race detector
// says so.
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

// TestNoInterleavingAcrossClonedLoggers: a parent and its Child hold different
// Printer mutexes over the same writer. Each line must still arrive whole.
func TestNoInterleavingAcrossClonedLoggers(t *testing.T) {
	useFixedTime(t)

	var out lockedBuffer
	parent := New()
	parent.SetOutput(&out)
	parent.SetTimeFormat("")
	child := parent.Child("child")

	const goroutines, lines = 8, 300
	var wg sync.WaitGroup
	for g := range goroutines {
		l := parent
		if g%2 == 1 {
			l = child
		}
		wg.Go(func() {
			for i := range lines {
				l.Infof("g%d-%d", g, i)
			}
		})
	}
	wg.Wait()

	valid := regexp.MustCompile(`^\[INFO\] (child: )?g\d+-\d+$`)
	got := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(got) != goroutines*lines {
		t.Errorf("got %d lines, want %d", len(got), goroutines*lines)
	}
	for _, line := range got {
		if !valid.MatchString(line) {
			t.Fatalf("interleaved line: %q", line)
		}
	}
}

func TestCloneDoesNotShareHandlers(t *testing.T) {
	parent, _ := newTestLogger(t)
	// Three handlers leave spare capacity in the slice (len 3, cap 4), which
	// is what let a parent and a clone append into the same slot.
	for range 3 {
		parent.Handle(func(*Log) bool { return false })
	}

	child := parent.Clone()

	var parentSecond, childSecond bool
	child.Handle(func(*Log) bool { childSecond = true; return false })
	parent.Handle(func(*Log) bool { parentSecond = true; return false })

	parent.Info("parent")
	if !parentSecond {
		t.Error("the parent's second handler did not run: the child's Handle overwrote it")
	}
	child.Info("child")
	if !childSecond {
		t.Error("the child's second handler did not run: the parent's Handle overwrote it")
	}
}

// TestHandleWhileLogging is a race detector target: Handle and SetIDGenerator
// write under the lock, so logging must read the handler set and NewID under it.
func TestHandleWhileLogging(t *testing.T) {
	useFixedTime(t)
	l := New()
	l.SetOutput(io.Discard)

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 100 {
				l.Info("line")
			}
		})
	}
	wg.Go(func() {
		for range 100 {
			l.Handle(func(*Log) bool { return false })
		}
	})
	wg.Go(func() {
		for range 100 {
			l.SetIDGenerator(uuid.NewV7)
			l.SetIDGenerator(nil)
		}
	})
	wg.Wait()
}

// TestFatalExitsWhenDisabled runs itself as a subprocess: Fatal must exit with
// status 1 even when the logger's level is "disable", and print nothing.
func TestFatalExitsWhenDisabled(t *testing.T) {
	if os.Getenv("GOLOG_TEST_FATAL") == "1" {
		l := New()
		l.SetOutput(os.Stdout)
		l.SetLevel("disable")
		l.Fatal("boom")
		fmt.Println("still alive")
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestFatalExitsWhenDisabled$")
	cmd.Env = append(os.Environ(), "GOLOG_TEST_FATAL=1")
	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("Fatal at level disable: err = %v, want exit status 1; output:\n%s", err, out)
	}
	if strings.Contains(string(out), "boom") {
		t.Errorf("Fatal at level disable printed its message:\n%s", out)
	}
}

func TestPlainFieldsAreSorted(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")

	for range 20 {
		buf.Reset()
		l.Info("m", Fields{"zeta": 1, "alpha": 2, "mid": 3})
		if got, want := buf.String(), "[INFO] m alpha=2 mid=3 zeta=1\n"; got != want {
			t.Fatalf("output = %q, want %q", got, want)
		}
	}
}

// getterFormatter calls back into its Logger, which a Formatter may reasonably
// do (the README's JSON handler example does the same from a Handler).
type getterFormatter struct{}

func (getterFormatter) String() string             { return "getter" }
func (f getterFormatter) Options(...any) Formatter { return f }
func (getterFormatter) Format(w io.Writer, l *Log) bool {
	_ = l.Logger.GetLevelOutput("info")
	_ = l.Logger.Child("audit")
	fmt.Fprintf(w, "ok:%s\n", l.Message)
	return true
}

func TestFormatterMayCallLoggerGetters(t *testing.T) {
	l, buf := newTestLogger(t)
	l.RegisterFormatter(getterFormatter{})
	l.SetFormat("getter")

	done := make(chan struct{})
	go func() {
		l.Info("x")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("a Formatter calling GetLevelOutput/Child deadlocked the logger")
	}
	if got, want := buf.String(), "ok:x\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// errReader yields its data and then a non-EOF error.
type errReader struct {
	data string
	err  error
	done bool
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), nil
}

func TestScanLongLineArrivesWhole(t *testing.T) {
	useFixedTime(t)
	var out lockedBuffer
	l := New()
	l.SetOutput(&out)
	l.SetTimeFormat("")

	long := strings.Repeat("x", 70<<10) // over bufio.Scanner's 64KB default cap.
	cancel := l.Scan(strings.NewReader(long + "\nafter\n"))
	defer cancel()

	waitFor(t, func() bool { return strings.Contains(out.String(), "after") })
	if got := out.String(); got != long+"\nafter\n" {
		t.Errorf("long line lost or split: got %d bytes, want %d", len(got), len(long)+7)
	}
}

func TestScanTruncatesHugeLine(t *testing.T) {
	useFixedTime(t)
	var out lockedBuffer
	l := New()
	l.SetOutput(&out)
	l.SetTimeFormat("")

	huge := strings.Repeat("y", 2<<20)
	cancel := l.Scan(strings.NewReader(huge + "\nafter\n"))
	defer cancel()

	waitFor(t, func() bool { return strings.Contains(out.String(), "after") })
	got := out.String()
	first, _, _ := strings.Cut(got, "\n")
	if !strings.HasSuffix(first, scanTruncated) {
		t.Errorf("huge line was not marked truncated (len %d)", len(first))
	}
	if len(first) > maxScanLine+len(scanTruncated) {
		t.Errorf("truncated line is %d bytes, cap is %d", len(first), maxScanLine)
	}
}

func TestScanReportsReadError(t *testing.T) {
	useFixedTime(t)
	var out lockedBuffer
	l := New()
	l.SetOutput(&out)
	l.SetTimeFormat("")

	cancel := l.Scan(&errReader{data: "partial\n", err: errors.New("disk on fire")})
	defer cancel()

	waitFor(t, func() bool { return strings.Contains(out.String(), "disk on fire") })
	if got := out.String(); !strings.Contains(got, "[ERRO] scan: disk on fire") {
		t.Errorf("read error not reported through the logger: %q", got)
	}
}

func TestScanGoesThroughPrefixAndHandlers(t *testing.T) {
	useFixedTime(t)
	var out lockedBuffer
	l := New()
	l.SetOutput(&out)
	l.SetPrefix("child: ")

	var seen lockedBuffer
	l.Handle(func(log *Log) bool { seen.Write([]byte(log.Message)); return false })

	cancel := l.Scan(strings.NewReader("hello\r\n"))
	defer cancel()

	waitFor(t, func() bool { return strings.Contains(out.String(), "hello") })
	if got, want := out.String(), "2020/06/06 06:04 child: hello\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if got := seen.String(); got != "hello" {
		t.Errorf("handler saw %q, want %q (CR must be trimmed)", got, "hello")
	}
}

func TestJSONFormatterMarshalErrorStaysJSON(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")
	l.SetFormat("json", "")

	l.Info("ratio", Fields{"nan": math.NaN(), "ok": 1})

	got := buf.String()
	if !strings.HasPrefix(got, "{") || !strings.HasSuffix(got, "}\n") {
		t.Fatalf("a marshal failure produced a non-JSON line: %q", got)
	}
	if !strings.Contains(got, `"golog_error"`) || !strings.Contains(got, `"message":"ratio"`) {
		t.Errorf("error record missing the message or the error: %q", got)
	}
}

// TestSetLevelOutputSharedBetweenParentAndChildJSON writes JSON from a parent
// and a child into one bare bytes.Buffer set with SetLevelOutput. Clone copies
// LevelOutput, so the two share the writer; the shared write lock is what
// keeps this from being a data race (which -race would report) and the lines
// from being corrupted.
func TestSetLevelOutputSharedBetweenParentAndChildJSON(t *testing.T) {
	useFixedTime(t)
	var out bytes.Buffer
	parent := New()
	parent.SetOutput(io.Discard)
	parent.SetTimeFormat("")
	parent.SetFormat("json", "")
	parent.SetLevelOutput("info", &out)
	child := parent.Child("c")

	var wg sync.WaitGroup
	for g := range 8 {
		l := parent
		if g%2 == 1 {
			l = child
		}
		wg.Go(func() {
			for i := range 200 {
				l.Infof("g%d-%d", g, i)
			}
		})
	}
	wg.Wait()

	for line := range strings.SplitSeq(strings.TrimSuffix(out.String(), "\n"), "\n") {
		if !strings.HasPrefix(line, `{"level":"info"`) || !strings.HasSuffix(line, "}") {
			t.Fatalf("corrupted JSON line: %q", line)
		}
	}
}

func TestParseLevelAliases(t *testing.T) {
	// An unknown name parses as DisableLevel anyway, so the assertion that
	// matters is that these are registered names, not accidents.
	for _, name := range []string{"disabled", "none", "off"} {
		if !slices.Contains(Levels[DisableLevel].AlternativeNames, name) {
			t.Errorf("%q is not an alternative name of DisableLevel", name)
		}
		if got := ParseLevel(strings.ToUpper(name)); got != DisableLevel {
			t.Errorf("ParseLevel(%q) = %v, want DisableLevel", name, got)
		}
	}
	if got := ParseLevel("warning"); got != WarnLevel {
		t.Errorf("ParseLevel(warning) = %v, want WarnLevel", got)
	}
}

// customLevel is unused by any other test, so registering it here cannot
// affect the golden outputs elsewhere.
const customLevel Level = 42

func TestRegisterLevelWhileLogging(t *testing.T) {
	useFixedTime(t)
	l := New()
	l.SetOutput(io.Discard)
	l.SetLevel("debug")

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 200 {
				l.Info("line")
				_ = InfoLevel.String()
				_ = ParseLevel("info")
			}
		})
	}
	wg.Go(func() {
		for i := range 200 {
			RegisterLevel(customLevel, &LevelMetadata{Name: "custom", Title: fmt.Sprintf("[C%d]", i)})
		}
	})
	wg.Wait()

	t.Cleanup(func() { delete(Levels, customLevel); ReloadLevels() })

	if got := customLevel.String(); got != "custom" {
		t.Errorf("customLevel.String() = %q after RegisterLevel, want %q", got, "custom")
	}
	if got := ParseLevel("custom"); got != customLevel {
		t.Errorf("ParseLevel(custom) = %v, want %v", got, customLevel)
	}
}

func TestSetTextIsSeenAfterFirstLine(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")

	l.Warn("before") // the snapshot exists from here on.

	meta := Levels[WarnLevel]
	prevTitle, prevColor, prevStyle := meta.Title, meta.ColorCode, meta.Style
	t.Cleanup(func() { meta.SetText(prevTitle, prevColor, prevStyle...) })

	meta.SetText("|W|", meta.ColorCode)
	buf.Reset()
	l.Warn("after")

	if got, want := buf.String(), "|W| after\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestDirectLevelsWriteNeedsReload(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")
	l.Level = customLevel // above debug, or nothing at customLevel is written.

	// A level added to the map directly is picked up on first use, because a
	// lookup miss rebuilds the snapshot.
	Levels[customLevel] = &LevelMetadata{Name: "direct", Title: "[DIR]"}
	t.Cleanup(func() { delete(Levels, customLevel); ReloadLevels() })

	l.Log(customLevel, "found")
	if got, want := buf.String(), "[DIR] found\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}

	// A change to an entry already in the snapshot is not, until ReloadLevels.
	Levels[customLevel].Title = "[NEW]"
	buf.Reset()
	l.Log(customLevel, "stale")
	if got, want := buf.String(), "[DIR] stale\n"; got != want {
		t.Fatalf("output = %q, want %q (a direct field write must not be seen before a reload)", got, want)
	}

	ReloadLevels()
	buf.Reset()
	l.Log(customLevel, "fresh")
	if got, want := buf.String(), "[NEW] fresh\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestIsStdlibFrame(t *testing.T) {
	tests := map[string]bool{
		"runtime.Callers":        true,
		"net/http.(*conn).serve": true,
		"testing.tRunner":        true,
		"vendor/golang.org/x/net/http2/hpack.(*Decoder).Write": true,
		"main.main":                                         false,
		"main.(*server).handle":                             false,
		"command-line-arguments.helper":                     false,
		"github.com/kataras/golog.(*Logger).print":          false,
		"github.com/kataras/iris/v12.(*Application).Listen": false,
		"gopath.local/app/handlers.Serve":                   false,
	}

	for function, want := range tests {
		if got := isStdlibFrame(function); got != want {
			t.Errorf("isStdlibFrame(%q) = %v, want %v", function, got, want)
		}
	}
}

func TestStacktraceSkipsGologButKeepsTests(t *testing.T) {
	l, _ := newTestLogger(t)

	var frames []Frame
	l.Handle(func(log *Log) bool {
		frames = slices.Clone(log.Stacktrace)
		return true
	})
	l.Debug("stack")

	if len(frames) == 0 {
		t.Fatal("no frames")
	}
	if !strings.Contains(frames[0].Source, "review_test.go") {
		t.Errorf("first frame should be this test, got %q", frames[0].Source)
	}
	for _, f := range frames {
		if strings.Contains(f.Source, "logger.go") || strings.Contains(f.Source, "log.go:") {
			t.Errorf("golog's own frame leaked into the stacktrace: %q", f.Source)
		}
	}
}

func TestLevelOutputColorDecidedOnce(t *testing.T) {
	l, _ := newTestLogger(t)
	l.SetTimeFormat("")

	var out bytes.Buffer
	l.SetLevelOutput("error", &out)
	if rich, ok := l.levelRich[ErrorLevel]; !ok || rich {
		t.Errorf("levelRich[error] = %v, %v; want false, true", rich, ok)
	}

	l.Error("plain")
	if got, want := out.String(), "[ERRO] plain\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
