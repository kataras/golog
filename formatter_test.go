package golog

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"
)

// fixedTime is what Now returns for the duration of a test that calls
// useFixedTime, so the "timestamp" field is a constant in the goldens.
var fixedTime = time.Date(2020, time.June, 6, 6, 4, 37, 0, time.UTC)

func useFixedTime(t *testing.T) {
	t.Helper()

	prev := Now
	Now = func() time.Time { return fixedTime }
	t.Cleanup(func() { Now = prev })
}

// newTestLogger returns a logger writing to a buffer, with the time frozen and
// colors out of the picture (a bytes.Buffer is not a terminal).
func newTestLogger(t *testing.T) (*Logger, *bytes.Buffer) {
	t.Helper()

	useFixedTime(t)

	var buf bytes.Buffer
	l := New()
	l.SetOutput(&buf)
	l.SetLevel("debug")
	return l, &buf
}

func TestJSONFormatterGolden(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Logger)
		write func(*Logger)
		want  string
	}{
		{
			name:  "info with fields",
			setup: func(l *Logger) { l.SetFormat("json") },
			write: func(l *Logger) { l.Info("hello", Fields{"username": "kataras"}) },
			want: "{\n" +
				"  \"timestamp\": 1591423477,\n" +
				"  \"level\": \"info\",\n" +
				"  \"message\": \"hello\",\n" +
				"  \"fields\": {\n" +
				"    \"username\": \"kataras\"\n" +
				"  }\n" +
				"}\n",
		},
		{
			name:  "custom indent",
			setup: func(l *Logger) { l.SetFormat("json", "    ") },
			write: func(l *Logger) { l.Warn("careful") },
			want: "{\n" +
				"    \"timestamp\": 1591423477,\n" +
				"    \"level\": \"warn\",\n" +
				"    \"message\": \"careful\"\n" +
				"}\n",
		},
		{
			name:  "empty indent is compact",
			setup: func(l *Logger) { l.SetFormat("json", "") },
			write: func(l *Logger) { l.Error("boom") },
			want:  `{"timestamp":1591423477,"level":"error","message":"boom"}` + "\n",
		},
		{
			// The regression this pins: under encoding/json/v2 "omitempty" no
			// longer omits a zero number, so a Timestamp of 0 would print as
			// "timestamp":0. The struct tag has to say "omitzero".
			name: "no time format omits the timestamp",
			setup: func(l *Logger) {
				l.SetTimeFormat("")
				l.SetFormat("json", "")
			},
			write: func(l *Logger) { l.Info("no time here") },
			want:  `{"level":"info","message":"no time here"}` + "\n",
		},
		{
			// v2 does not sort map keys unless Deterministic is set. Without it
			// this test fails intermittently, which is the worst way to find out.
			name:  "fields are ordered deterministically",
			setup: func(l *Logger) { l.SetFormat("json", "") },
			write: func(l *Logger) {
				l.Info("ordered", Fields{"zeta": 3, "alpha": 1, "mu": 2})
			},
			want: `{"timestamp":1591423477,"level":"info","message":"ordered","fields":{"alpha":1,"mu":2,"zeta":3}}` + "\n",
		},
		{
			// v2 rejects invalid UTF-8 by default, which for a logger means
			// returning an error and dropping the line. AllowInvalidUTF8 keeps
			// v1's behavior of substituting U+FFFD.
			name:  "invalid utf-8 is replaced, not rejected",
			setup: func(l *Logger) { l.SetFormat("json", "") },
			write: func(l *Logger) { l.Info("bad \xff byte") },
			want:  `{"timestamp":1591423477,"level":"info","message":"bad � byte"}` + "\n",
		},
		{
			// The one deliberate output change of the v1 -> v2 migration.
			// v1 wrote <b> and & here.
			name:  "html characters are no longer escaped",
			setup: func(l *Logger) { l.SetFormat("json", "") },
			write: func(l *Logger) { l.Info(`<b>a & b</b>`) },
			want:  `{"timestamp":1591423477,"level":"info","message":"<b>a & b</b>"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, buf := newTestLogger(t)
			tt.setup(l)
			tt.write(l)

			if got := buf.String(); got != tt.want {
				t.Errorf("output mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestJSONFormatterID(t *testing.T) {
	t.Run("omitted when no generator", func(t *testing.T) {
		l, buf := newTestLogger(t)
		l.SetFormat("json", "")
		l.Info("no id")

		if strings.Contains(buf.String(), `"id"`) {
			t.Fatalf("id should be omitted when NewID is nil, got %q", buf.String())
		}
	})

	t.Run("present when generator set", func(t *testing.T) {
		l, buf := newTestLogger(t)
		l.SetFormat("json", "")

		id := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
		l.SetIDGenerator(func() uuid.UUID { return id })
		l.Info("with id")

		want := `{"id":"f81d4fae-7dec-11d0-a765-00a0c91e6bf6","timestamp":1591423477,"level":"info","message":"with id"}` + "\n"
		if got := buf.String(); got != want {
			t.Fatalf("output mismatch\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("cleared again when generator removed", func(t *testing.T) {
		// Logs are pooled. A Log that carried an id must not keep printing it
		// after the generator is taken away.
		l, buf := newTestLogger(t)
		l.SetFormat("json", "")
		l.SetIDGenerator(uuid.NewV7)
		l.Info("first")

		buf.Reset()
		l.SetIDGenerator(nil)
		l.Info("second")

		if strings.Contains(buf.String(), `"id"`) {
			t.Fatalf("stale id survived in the pool: %q", buf.String())
		}
	})
}

// TestJSONFormatterLevelOutputAfterFirstWrite pins the bug the pooled
// jsontext.Encoder fixed: the old formatter cached one encoder per level, bound
// to whatever writer that level had at the time, so a SetLevelOutput after the
// first log at that level kept writing to the old writer forever.
func TestJSONFormatterLevelOutputAfterFirstWrite(t *testing.T) {
	l, first := newTestLogger(t)
	l.SetFormat("json", "")

	l.Info("to the first writer")
	if first.Len() == 0 {
		t.Fatal("nothing written to the original output")
	}

	var second bytes.Buffer
	l.SetLevelOutput("info", &second)
	first.Reset()

	l.Info("to the second writer")

	if first.Len() != 0 {
		t.Errorf("still writing to the old writer: %q", first.String())
	}
	if !strings.Contains(second.String(), "to the second writer") {
		t.Errorf("new writer did not receive the log, got %q", second.String())
	}
}

// TestSetLevelFormatPerEntryLevel pins the getFormatter fix: the per-level
// formatter has to be chosen by the level of the entry, not by the logger's
// own threshold.
func TestSetLevelFormatPerEntryLevel(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")
	l.SetLevelFormat("warn", "json", "")

	l.Warn("as json")
	if got, want := buf.String(), `{"level":"warn","message":"as json"}`+"\n"; got != want {
		t.Errorf("warn should use the json formatter\n got: %q\nwant: %q", got, want)
	}

	buf.Reset()
	l.Info("as plain text")
	if got := buf.String(); strings.HasPrefix(got, "{") {
		t.Errorf("info should not use the warn formatter, got %q", got)
	}
}

func TestJSONFormatterOptionsDefaults(t *testing.T) {
	f := new(JSONFormatter)

	if got := f.String(); got != "json" {
		t.Fatalf("String() = %q, want %q", got, "json")
	}

	if got := f.Options().(*JSONFormatter).Indent; got != "  " {
		t.Errorf("default Indent = %q, want two spaces", got)
	}
	if got := f.Options("\t").(*JSONFormatter).Indent; got != "\t" {
		t.Errorf("Indent = %q, want a tab", got)
	}
	if got := f.Options(nil, 42, "  ").(*JSONFormatter).Indent; got != "  " {
		t.Errorf("Indent = %q, non-string options should be skipped", got)
	}
}

// A JSONFormatter built directly, rather than through Options, used to panic on
// its nil encoders map. It has to keep working.
func TestJSONFormatterZeroValue(t *testing.T) {
	useFixedTime(t)

	var buf bytes.Buffer
	l := New()
	l.SetTimeFormat("")

	log := &Log{Logger: l, Level: InfoLevel, Message: "direct"}
	if ok := new(JSONFormatter).Format(&buf, log); !ok {
		t.Fatal("Format reported failure")
	}

	if got, want := buf.String(), `{"level":"info","message":"direct"}`+"\n"; got != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestJSONFormatterUnknownLevelFails(t *testing.T) {
	useFixedTime(t)

	var buf bytes.Buffer
	l := New()
	l.SetTimeFormat("")

	log := &Log{Logger: l, Level: Level(99), Message: "who?"}
	if ok := new(JSONFormatter).Format(&buf, log); ok {
		t.Error("Format should report failure for a level with no metadata")
	}
}

// countingWriter records how many times it was written to, and what it got.
type countingWriter struct {
	writes int
	buf    bytes.Buffer
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.writes++
	return c.buf.Write(p)
}

// TestJSONFormatterSingleWrite pins the buffering in Format. Handing the
// destination to jsontext directly makes it flush whenever its scratch buffer
// passes three quarters full, which is two Write calls per line -- two chances
// for a concurrent logger sharing the writer to cut in mid-line.
func TestJSONFormatterSingleWrite(t *testing.T) {
	useFixedTime(t)

	l := New()
	l.SetTimeFormat("")
	f := new(JSONFormatter).Options("  ")

	for _, size := range []int{0, 100, 4096, 100000} {
		msg := strings.Repeat("x", size)
		log := &Log{Logger: l, Level: InfoLevel, Message: msg}

		w := &countingWriter{}
		if ok := f.Format(w, log); !ok {
			t.Fatalf("Format failed for a %d byte message", size)
		}
		if w.writes != 1 {
			t.Errorf("message of %d bytes took %d Write calls, want exactly 1", size, w.writes)
		}
	}
}

// TestJSONFormatterDurationField pins the reason this formatter encodes with
// DefaultOptionsV1. Under encoding/json/v2's own defaults a time.Duration has
// "no default representation", so Format would fail, drop the line to the
// plain-text fallback, and (before the buffering) leave a truncated JSON
// fragment on the output first.
func TestJSONFormatterDurationField(t *testing.T) {
	l, buf := newTestLogger(t)
	l.SetTimeFormat("")
	l.SetFormat("json", "")

	l.Info("done", Fields{"took": 1500 * time.Millisecond})

	want := `{"level":"info","message":"done","fields":{"took":1500000000}}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", got, want)
	}
}

// TestJSONFormatterV1ValueRendering covers the rest of the v1 renderings that
// encoding/json/v2's defaults would have changed for values a caller put in
// Fields. Each of these is somebody else's data, so golog does not get to
// change how it looks.
func TestJSONFormatterV1ValueRendering(t *testing.T) {
	type inner struct {
		Count int    `json:"count,omitempty"`
		Flag  bool   `json:"flag,omitempty"`
		Name  string `json:"name,omitempty"`
	}

	var (
		nilSlice []int
		nilMap   map[string]int
	)

	l, buf := newTestLogger(t)
	l.SetTimeFormat("")
	l.SetFormat("json", "")

	l.Info("values", Fields{
		"a_omitempty": inner{},
		"b_bytearray": [3]byte{1, 2, 3},
		"c_nilslice":  nilSlice,
		"d_nilmap":    nilMap,
	})

	want := `{"level":"info","message":"values","fields":{` +
		`"a_omitempty":{},` + // v2 alone would emit count and flag
		`"b_bytearray":[1,2,3],` + // v2 alone would base64 this
		`"c_nilslice":null,` + // v2 alone would emit []
		`"d_nilmap":null` + // v2 alone would emit {}
		`}}` + "\n"

	if got := buf.String(); got != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", got, want)
	}
}

// TestJSONFormatterAllocations guards the claim this migration is built on.
// The pooled encoder plus Level.MarshalJSONTo should keep a steady-state format
// free of allocations beyond the message itself.
func TestJSONFormatterAllocations(t *testing.T) {
	if raceEnabled {
		// Under -race, sync.Pool.Put drops one item in four on purpose, and
		// every drop makes the next Get build a fresh encoder and buffer. The
		// steady state this test measures does not exist in that build.
		t.Skip("sync.Pool discards items at random under the race detector")
	}
	useFixedTime(t)

	l := New()
	l.SetTimeFormat("")
	f := new(JSONFormatter).Options("")
	log := &Log{Logger: l, Level: InfoLevel, Message: "steady state"}

	got := testing.AllocsPerRun(200, func() {
		f.Format(io.Discard, log)
	})

	const want = 1 // the io.Discard interface conversion of the buffer bytes.
	if got > want {
		t.Errorf("Format allocates %v times per call, want at most %d", got, want)
	}
}

// TestJSONFormatterConcurrent runs the formatter from many goroutines at once.
// The old implementation serialized every encode through a single package-level
// mutex; the pool has to be correct without it.
func TestJSONFormatterConcurrent(t *testing.T) {
	useFixedTime(t)

	l := New()
	l.SetTimeFormat("")
	f := new(JSONFormatter).Options("")

	var mu sync.Mutex
	lines := make(map[string]int)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 100 {
				var buf bytes.Buffer
				log := &Log{
					Logger:  l,
					Level:   InfoLevel,
					Message: fmt.Sprintf("goroutine %d line %d", i, j),
				}
				if ok := f.Format(&buf, log); !ok {
					t.Errorf("Format failed")
					return
				}

				mu.Lock()
				lines[buf.String()]++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	if got, want := len(lines), 8*100; got != want {
		t.Errorf("got %d distinct lines, want %d (lines were corrupted or duplicated)", got, want)
	}
	for line, n := range lines {
		if n != 1 {
			t.Errorf("line %q written %d times", line, n)
		}
		if !strings.HasSuffix(line, "}\n") {
			t.Errorf("line %q is not a complete JSON object", line)
		}
	}
}
