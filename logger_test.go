package golog

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"
)

func TestPlainOutput(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Logger)
		write func(*Logger)
		want  string
	}{
		{
			name:  "level title, time and message",
			write: func(l *Logger) { l.Info("hello") },
			want:  "[INFO] 2020/06/06 06:04 hello\n",
		},
		{
			name:  "no time format",
			setup: func(l *Logger) { l.SetTimeFormat("") },
			write: func(l *Logger) { l.Warn("careful") },
			want:  "[WARN] careful\n",
		},
		{
			name:  "prefix sits between the time and the message",
			setup: func(l *Logger) { l.SetTimeFormat(""); l.SetPrefix("api: ") },
			write: func(l *Logger) { l.Error("boom") },
			want:  "[ERRO] api: boom\n",
		},
		{
			name:  "fields are appended as key=value",
			setup: func(l *Logger) { l.SetTimeFormat("") },
			write: func(l *Logger) { l.Info("hi", Fields{"user": "kataras"}) },
			want:  "[INFO] hi user=kataras\n",
		},
		{
			name:  "Print carries no level and no color",
			setup: func(l *Logger) { l.SetTimeFormat("") },
			write: func(l *Logger) { l.Print("raw") },
			want:  "raw\n",
		},
		{
			name:  "DisableNewLine drops the trailing newline",
			setup: func(l *Logger) { l.SetTimeFormat(""); l.DisableNewLine() },
			write: func(l *Logger) { l.Info("no newline") },
			want:  "[INFO] no newline",
		},
		{
			name:  "Infof formats",
			setup: func(l *Logger) { l.SetTimeFormat("") },
			write: func(l *Logger) { l.Infof("%s=%d", "n", 42) },
			want:  "[INFO] n=42\n",
		},
		{
			name:  "Warningf is an alias of Warnf",
			setup: func(l *Logger) { l.SetTimeFormat("") },
			write: func(l *Logger) { l.Warningf("%s", "aliased") },
			want:  "[WARN] aliased\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, buf := newTestLogger(t)
			if tt.setup != nil {
				tt.setup(l)
			}
			tt.write(l)

			if got := buf.String(); got != tt.want {
				t.Errorf("output mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestPlainOutputID(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")

	id := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	l.SetIDGenerator(func() uuid.UUID { return id })
	l.Info("tagged")

	want := "[INFO] f81d4fae-7dec-11d0-a765-00a0c91e6bf6 tagged\n"
	if got := buf.String(); got != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestLevelThreshold(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")
	l.SetLevel("warn")

	l.Debug("debug")
	l.Info("info")
	l.Warn("warn")
	l.Error("error")

	got := buf.String()
	for _, unwanted := range []string{"debug", "info"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("%q should have been filtered out by the warn level, got %q", unwanted, got)
		}
	}
	for _, wanted := range []string{"warn", "error"} {
		if !strings.Contains(got, wanted) {
			t.Errorf("%q should have been printed at the warn level, got %q", wanted, got)
		}
	}
}

func TestDebugCollectsStacktrace(t *testing.T) {
	l, _ := newTestLogger(t)
	l.SetStacktraceLimit(3)

	var frames []Frame
	l.Handle(func(log *Log) bool {
		frames = log.Stacktrace
		return true
	})

	l.Debug("with a stack")

	if len(frames) == 0 {
		t.Fatal("no stacktrace collected on the debug level")
	}
	if len(frames) > 3 {
		t.Errorf("StacktraceLimit(3) not respected, got %d frames", len(frames))
	}
	if frames[0].Source == "" || !strings.Contains(frames[0].Source, ":") {
		t.Errorf("Source should be file:line, got %q", frames[0].Source)
	}
	if frames[0].String() != frames[0].Source {
		t.Errorf("Frame.String() = %q, want %q", frames[0].String(), frames[0].Source)
	}
}

func TestNegativeStacktraceLimitDisablesIt(t *testing.T) {
	if got := GetStacktrace(-1); got != nil {
		t.Errorf("GetStacktrace(-1) = %v, want nil", got)
	}
}

func TestFieldsGet(t *testing.T) {
	f := Fields{"name": "kataras", "age": 33, "nil": nil}

	if got, ok := f.Get[string]("name"); !ok || got != "kataras" {
		t.Errorf(`Get[string]("name") = %q, %v; want "kataras", true`, got, ok)
	}
	if got, ok := f.Get[int]("age"); !ok || got != 33 {
		t.Errorf(`Get[int]("age") = %d, %v; want 33, true`, got, ok)
	}
	if got, ok := f.Get[string]("age"); ok || got != "" {
		t.Errorf(`Get[string]("age") = %q, %v; want "", false (wrong type)`, got, ok)
	}
	if got, ok := f.Get[string]("missing"); ok || got != "" {
		t.Errorf(`Get[string]("missing") = %q, %v; want "", false`, got, ok)
	}
	if _, ok := f.Get[string]("nil"); ok {
		t.Error(`Get[string]("nil") should report false for a nil value`)
	}
	if got, ok := f.Get[any]("name"); !ok || got != "kataras" {
		t.Errorf(`Get[any]("name") = %v, %v; want "kataras", true`, got, ok)
	}
}

func TestLogField(t *testing.T) {
	l, _ := newTestLogger(t)

	var (
		user string
		ok   bool
	)
	l.Handle(func(log *Log) bool {
		user, ok = log.Field[string]("username")
		return true
	})

	l.Info("hi", Fields{"username": "kataras"})

	if !ok || user != "kataras" {
		t.Errorf(`log.Field[string]("username") = %q, %v; want "kataras", true`, user, ok)
	}
}

func TestSplitArgsFieldsAcceptsSlogAttrs(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")
	l.SetFormat("json", "")

	l.Info("attrs",
		slog.String("single", "one"),
		[]slog.Attr{slog.Int("many", 2)},
		Fields{"plain": true},
	)

	want := `{"level":"info","message":"attrs","fields":{"many":2,"plain":true,"single":"one"}}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestChildPrefixes(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")

	child := l.Child("api")
	child.Info("from the child")

	if got, want := buf.String(), "[INFO] api: from the child\n"; got != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", got, want)
	}

	// Child is get-or-create: the same key must return the same logger rather
	// than appending the prefix a second time.
	if same := l.Child("api"); same != child {
		t.Error("Child returned a different logger for the same key")
	}

	grandchild := child.Child("users")
	buf.Reset()
	grandchild.Info("nested")

	if got, want := buf.String(), "[INFO] api: users: from the child\n"; got == want {
		t.Fatal("unreachable: prefixes are appended, not replaced")
	}
	if got := buf.String(); !strings.HasPrefix(got, "[INFO] api: users: ") {
		t.Errorf("grandchild prefix = %q, want it to start with %q", got, "[INFO] api: users: ")
	}
}

// TestChildOrdering pins the bug that remove() used to have: registration order
// lived in a map[int]any and was rebuilt by ranging over it, and Go randomizes
// map iteration order, so removing a child scrambled the order of the rest.
func TestChildOrdering(t *testing.T) {
	l, _ := newTestLogger(t)

	keys := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	for _, k := range keys {
		l.Child(k)
	}

	if got, want := l.ChildCount(), len(keys); got != want {
		t.Fatalf("ChildCount() = %d, want %d", got, want)
	}
	if got := l.ListChildKeys(); !equalKeys(got, keys) {
		t.Fatalf("ListChildKeys() = %v, want %v", got, keys)
	}
	if l.LastChild() != l.Child("h") {
		t.Error("LastChild() is not the last registered child")
	}

	if !l.RemoveChild("c") {
		t.Fatal(`RemoveChild("c") reported false`)
	}
	if l.RemoveChild("c") {
		t.Error(`RemoveChild("c") twice should report false the second time`)
	}

	want := []string{"a", "b", "d", "e", "f", "g", "h"}
	if got := l.ListChildKeys(); !equalKeys(got, want) {
		t.Errorf("order was not preserved across a removal\n got: %v\nwant: %v", got, want)
	}
	if l.LastChild() != l.Child("h") {
		t.Error("LastChild() changed after removing a middle child")
	}

	// Removing the last one has to move LastChild back, not off the end.
	l.RemoveChild("h")
	if l.LastChild() != l.Child("g") {
		t.Error("LastChild() should be g after removing h")
	}

	l.ClearChildren()
	if got := l.ChildCount(); got != 0 {
		t.Errorf("ChildCount() after ClearChildren = %d, want 0", got)
	}
	if l.LastChild() != nil {
		t.Error("LastChild() should be nil with no children")
	}
	if got := l.ListChildKeys(); len(got) != 0 {
		t.Errorf("ListChildKeys() after ClearChildren = %v, want empty", got)
	}
}

func equalKeys(got []any, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i, k := range want {
		if got[i] != any(k) {
			return false
		}
	}
	return true
}

// stringerKey exercises the fmt.Stringer branch of loggerMap.getOrAdd.
type stringerKey string

func (s stringerKey) String() string { return string(s) }

func TestChildStringerKey(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")

	l.Child(stringerKey("worker")).Info("running")

	if got, want := buf.String(), "[INFO] worker: from"; strings.HasPrefix(got, want) {
		t.Fatal("unreachable")
	}
	if got := buf.String(); !strings.HasPrefix(got, "[INFO] worker: ") {
		t.Errorf("output = %q, want a %q prefix", got, "[INFO] worker: ")
	}
}

func TestChildInheritsIDGenerator(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")

	id := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	l.SetIDGenerator(func() uuid.UUID { return id })

	l.Child("api").Info("inherited")

	if got := buf.String(); !strings.Contains(got, id.String()) {
		t.Errorf("child did not inherit the id generator, got %q", got)
	}
}

func TestClone(t *testing.T) {
	l, _ := newTestLogger(t)
	l.SetPrefix("orig: ")
	l.SetLevel("error")
	l.SetIDGenerator(uuid.NewV7)

	c := l.Clone()
	if c == l {
		t.Fatal("Clone returned the same logger")
	}
	if c.Prefix != l.Prefix || c.Level != l.Level || c.TimeFormat != l.TimeFormat {
		t.Error("Clone did not copy the scalar fields")
	}
	if c.NewID == nil {
		t.Error("Clone did not copy NewID")
	}

	// The clone's own children must not be the original's.
	c.Child("only-on-the-clone")
	if l.ChildCount() != 0 {
		t.Error("a child added to the clone leaked into the original")
	}
}

func TestSetLevelOutput(t *testing.T) {
	l, main := newTestLogger(t)
	l.SetTimeFormat("")

	var errors bytes.Buffer
	l.SetLevelOutput("error", &errors)

	l.Info("to main")
	l.Error("to errors")

	if got := main.String(); !strings.Contains(got, "to main") || strings.Contains(got, "to errors") {
		t.Errorf("main output = %q", got)
	}
	if got := errors.String(); !strings.Contains(got, "to errors") {
		t.Errorf("error output = %q, want it to contain %q", got, "to errors")
	}

	if got := l.GetLevelOutput("error"); got != io.Writer(&errors) {
		t.Error("GetLevelOutput did not return the registered writer")
	}
	// Never nil: an unregistered level falls back to the Printer.
	if got := l.GetLevelOutput("info"); got == nil {
		t.Error("GetLevelOutput returned nil for an unregistered level")
	}
}

func TestAddOutput(t *testing.T) {
	l, first := newTestLogger(t)
	l.SetTimeFormat("")

	var second bytes.Buffer
	l.AddOutput(&second)
	l.Info("to both")

	if !strings.Contains(first.String(), "to both") {
		t.Errorf("first writer = %q", first.String())
	}
	if !strings.Contains(second.String(), "to both") {
		t.Errorf("second writer = %q", second.String())
	}
}

func TestScan(t *testing.T) {
	useFixedTime(t)

	// A lockedBuffer, not the bare bytes.Buffer of newTestLogger: the scan
	// goroutine writes while this goroutine polls, and the race detector
	// rightly flagged the unsynchronized bytes.Buffer.
	var buf lockedBuffer
	l := New()
	l.SetOutput(&buf)
	l.SetTimeFormat("")

	pr, pw := io.Pipe()
	cancel := l.Scan(pr)

	done := make(chan struct{})
	l.Handle(func(*Log) bool { return false })

	go func() {
		defer close(done)
		fmt.Fprintln(pw, "scanned line")
		pw.Close()
	}()
	<-done

	// The scan goroutine writes asynchronously; wait for the content to land.
	waitFor(t, func() bool { return strings.Contains(buf.String(), "scanned line") })

	// Cancelling after the reader is already exhausted used to panic with
	// "close of closed channel": the scan goroutine closed the stop channel on
	// its way out and cancel closed it again, guarded only by a select that is
	// a check-then-act. Calling it repeatedly, and from several goroutines at
	// once, has to be safe.
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(cancel)
	}
	wg.Wait()
	cancel()
}

func TestInstall(t *testing.T) {
	t.Run("slog", func(t *testing.T) {
		l, _ := newTestLogger(t)

		var out bytes.Buffer
		l.Install(slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{
			Level: slog.LevelDebug,
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.Attr{}
				}
				return a
			},
		})))

		l.Info("through slog", Fields{"user": "kataras"})

		got := out.String()
		if !strings.Contains(got, "level=INFO") || !strings.Contains(got, `msg="through slog"`) {
			t.Errorf("slog output = %q", got)
		}
		if !strings.Contains(got, "user=kataras") {
			t.Errorf("fields did not reach slog: %q", got)
		}
	})

	t.Run("slog carries the id", func(t *testing.T) {
		l, _ := newTestLogger(t)

		id := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
		l.SetIDGenerator(func() uuid.UUID { return id })

		var out bytes.Buffer
		l.Install(slog.New(slog.NewTextHandler(&out, nil)))
		l.Info("tagged")

		if got := out.String(); !strings.Contains(got, "id="+id.String()) {
			t.Errorf("id did not reach slog: %q", got)
		}
	})

	t.Run("std log.Logger", func(t *testing.T) {
		l, _ := newTestLogger(t)

		var out bytes.Buffer
		l.Install(log.New(&out, "", 0))
		l.Info("through std")

		if got, want := out.String(), "through std\n"; got != want {
			t.Errorf("std output = %q, want %q", got, want)
		}
	})

	t.Run("ExternalLogger", func(t *testing.T) {
		l, _ := newTestLogger(t)

		ext := &recordingExternalLogger{}
		l.Install(ext)

		l.Error("an error")
		l.Warn("a warning")
		l.Info("an info")
		l.Debug("a debug")
		l.Println("a println")

		// Print routes to the external Println while the logger's NewLine is
		// on, because that is what it sets on the Log. Log.NewLine's own doc
		// claims a Print always clears it; the code has passed l.NewLine since
		// before this migration and consumers depend on the newline, so the
		// behavior stands and the test records it.
		l.Print("a print with NewLine on")
		l.DisableNewLine()
		l.Print("a print with NewLine off")

		want := []string{
			"Error:an error", "Warn:a warning", "Info:an info", "Debug:a debug",
			"Println:a println",
			"Println:a print with NewLine on",
			"Print:a print with NewLine off",
		}
		if !equalStrings(ext.calls, want) {
			t.Errorf("calls = %v, want %v", ext.calls, want)
		}
	})

	t.Run("unsupported panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("Install should panic on an unsupported logger")
			}
		}()

		New().Install(struct{}{})
	})
}

type recordingExternalLogger struct {
	calls []string
}

func (r *recordingExternalLogger) record(method string, v ...any) {
	r.calls = append(r.calls, method+":"+fmt.Sprint(v...))
}

func (r *recordingExternalLogger) Print(v ...any)   { r.record("Print", v...) }
func (r *recordingExternalLogger) Println(v ...any) { r.record("Println", v...) }
func (r *recordingExternalLogger) Error(v ...any)   { r.record("Error", v...) }
func (r *recordingExternalLogger) Warn(v ...any)    { r.record("Warn", v...) }
func (r *recordingExternalLogger) Info(v ...any)    { r.record("Info", v...) }
func (r *recordingExternalLogger) Debug(v ...any)   { r.record("Debug", v...) }

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestHandlerStopsTheChain(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")

	var second bool
	l.Handle(func(*Log) bool { return true })
	l.Handle(func(*Log) bool { second = true; return true })

	l.Info("handled")

	if second {
		t.Error("the second handler ran after the first returned true")
	}
	if buf.Len() != 0 {
		t.Errorf("a handled log should not reach the output, got %q", buf.String())
	}
}

func TestRegisterFormatter(t *testing.T) {
	l, buf := newTestLogger(t)
	l.RegisterFormatter(&plainFormatter{})
	l.SetFormat("plain")
	l.Info("custom")

	if got, want := buf.String(), "plain:custom\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

type plainFormatter struct{}

func (f *plainFormatter) String() string           { return "plain" }
func (f *plainFormatter) Options(...any) Formatter { return f }
func (f *plainFormatter) Format(w io.Writer, l *Log) bool {
	fmt.Fprintf(w, "plain:%s\n", l.Message)
	return true
}

// TestConcurrentLoggingWithOutputChanges is the race detector's target: writing
// from many goroutines while the writer set is being changed underneath them.
// Printer.WriteRich used to read writers and rich without holding the lock.
func TestConcurrentLoggingWithOutputChanges(t *testing.T) {
	useFixedTime(t)

	l := New()
	l.SetOutput(io.Discard)
	l.SetLevel("debug")

	const goroutines = 8

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() {
			for j := range 50 {
				l.Infof("goroutine %d line %d", i, j)
				l.Child(fmt.Sprintf("child-%d", i%3))
			}
		})
	}

	wg.Go(func() {
		for range 50 {
			l.AddOutput(io.Discard)
		}
	})
	wg.Go(func() {
		for range 50 {
			l.SetOutput(io.Discard)
		}
	})

	wg.Wait()
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
