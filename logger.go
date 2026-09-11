package golog

import (
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"uuid"

	"github.com/kataras/golog/printer"
)

// Handler is the signature type for logger's handler.
//
// A Handler can be used to intercept the message between a log value
// and the actual print operation, it's called
// when one of the print functions called.
// If it's return value is true then it means that the specific
// handler handled the log by itself therefore no need to
// proceed with the default behavior of printing the log
// to the specified logger's output.
//
// It stops on the handler which returns true firstly.
// The `Log` value holds the level of the print operation as well.
//
// The `*Log` is pooled and recycled as soon as the line has been written,
// so a Handler must not retain it (or its Fields and Stacktrace slices)
// beyond the call.
type Handler func(value *Log) (handled bool)

// Logger is our golog.
type Logger struct {
	Prefix     string
	Level      Level
	TimeFormat string
	// Limit stacktrace entries on `Debug` level.
	StacktraceLimit int
	// if new line should be added on all log functions, even the `F`s.
	// It defaults to true.
	//
	// See `golog#NewLine(newLineChar string)` as well.
	//
	// Note that this will not override the time and level prefix,
	// if you want to customize the log message please read the examples
	// or navigate to: https://github.com/kataras/golog/issues/3#issuecomment-355895870.
	NewLine bool
	// NewID, when not nil, is called once per log entry to fill Log.ID.
	//
	// It is nil by default, which leaves Log.ID the zero UUID and keeps "id"
	// out of the output entirely. See `SetIDGenerator`.
	NewID func() uuid.UUID
	// mu guards the configuration fields above and the maps below. It is
	// never held while writing to an output, so a Handler or a Formatter may
	// call any getter on its Logger.
	mu sync.RWMutex
	// outMu serializes writes to the per-level writers in LevelOutput, which
	// unlike the Printer have no lock of their own. It is shared with every
	// Clone (and therefore every Child), because Clone copies LevelOutput and
	// a parent and its children then write to the same writer.
	outMu   *sync.Mutex
	Printer *printer.Printer
	// LevelOutput holds the per level raw writers, optionally.
	//
	// Set entries through `SetLevelOutput`, which also decides once whether the
	// writer renders color. Writing to the map directly still works, but the
	// color check then runs on every line for that level, and the write has to
	// happen before logging starts because logging reads the map concurrently.
	LevelOutput map[Level]io.Writer
	// levelRich caches SupportsColor for the writers in LevelOutput, keyed the
	// same way; SetLevelOutput keeps the two in step.
	levelRich map[Level]bool

	formatters     map[string]Formatter // available formatters.
	formatter      Formatter            // the current formatter for all logs.
	LevelFormatter map[Level]Formatter  // per level formatter.

	handlers []Handler
	logs     sync.Pool
	children *loggerMap
}

// New returns a new golog with a default output to `os.Stdout`
// and level to `InfoLevel`.
func New() *Logger {
	return &Logger{
		Level:       InfoLevel,
		TimeFormat:  "2006/01/02 15:04",
		NewLine:     true,
		outMu:       new(sync.Mutex),
		Printer:     printer.NewPrinter(os.Stdout),
		LevelOutput: make(map[Level]io.Writer),
		levelRich:   make(map[Level]bool),
		formatters: map[string]Formatter{ // the available builtin formatters.
			"json": new(JSONFormatter),
		},
		LevelFormatter: make(map[Level]Formatter),
		children:       newLoggerMap(),
	}
}

// Fields is a map type.
// One or more values of `Fields` type can be passed
// on all Log methods except `Print/Printf/Println` to set the `Log.Fields` field,
// which can be accessed through a custom LogHandler.
type Fields map[string]any

// Get returns the value stored under "key" as a T.
//
// It reports false when the key is absent or holds something that is not a T,
// which saves Handler and Formatter authors the type assertion:
//
//	username, ok := log.Fields.Get[string]("username")
//
// Go 1.27 is what makes this a method rather than a package-level function:
// before generic methods, the type parameter could only have lived on a
// `golog.GetField[T](fields, key)`.
func (f Fields) Get[T any](key string) (T, bool) {
	t, ok := f[key].(T)
	return t, ok
}

// printConfig is the part of a Logger's configuration one log line needs,
// read once under the read lock so that the rest of the print runs without it.
type printConfig struct {
	prefix          string
	timeFormat      string
	newLine         bool
	stacktraceLimit int
	newID           func() uuid.UUID
	handlers        []Handler
	formatter       Formatter
	w               io.Writer
	// rich reports whether w renders color. Unused when w is a
	// *printer.Printer, which answers per writer on its own.
	rich bool
}

// snapshot reads the configuration for a log at "level". The caller holds mu.
func (l *Logger) snapshot(level Level) printConfig {
	cfg := printConfig{
		prefix:          l.Prefix,
		timeFormat:      l.TimeFormat,
		newLine:         l.NewLine,
		stacktraceLimit: l.StacktraceLimit,
		newID:           l.NewID,
		handlers:        l.handlers,
		formatter:       l.getFormatter(level),
	}

	if w, ok := l.LevelOutput[level]; ok {
		cfg.w = w
		if rich, ok := l.levelRich[level]; ok {
			cfg.rich = rich
		} else {
			// Written into the map directly rather than through SetLevelOutput,
			// so nothing decided this up front.
			cfg.rich = printer.SupportsColor(w)
		}
	} else {
		cfg.w = l.Printer
	}

	return cfg
}

// acquireLog returns a new log fom the pool.
func (l *Logger) acquireLog(level Level, msg string, withPrintln bool, fields Fields, cfg *printConfig) *Log {
	log, ok := l.logs.Get().(*Log)
	if !ok {
		log = &Log{
			Logger: l,
		}
	}

	log.NewLine = withPrintln
	if cfg.newID != nil {
		log.ID = cfg.newID()
	}
	log.timeFormat = cfg.timeFormat
	if cfg.timeFormat != "" {
		log.Time = Now()
		log.Timestamp = log.Time.Unix()
	}
	log.Level = level
	log.Message = msg
	log.Fields = fields
	return log
}

// maxPooledLogBuffer is the largest scratch line buffer worth keeping alive on
// a pooled Log. One 10MB message should not permanently cost 10MB of idle memory.
const maxPooledLogBuffer = 64 << 10

// releaseLog Log releases a log instance back to the pool.
//
// Every field is reset here rather than conditionally in acquireLog, so a
// recycled Log can never carry the previous entry's id, time, fields or
// stacktrace, and the pool does not pin a caller's Fields until the slot is
// reused.
func (l *Logger) releaseLog(log *Log) {
	buf := log.buf[:0]
	if cap(buf) > maxPooledLogBuffer {
		buf = nil
	}
	*log = Log{
		Logger:     l,
		Stacktrace: log.Stacktrace[:0],
		buf:        buf,
	}
	l.logs.Put(log)
}

// formatLog formats and writes the log entry to its output writer.
//
// The plain line is built in full into the Log's scratch buffer and written
// with a single Write. That is what keeps a line whole when several loggers
// share a destination, and it is one syscall per line on an unbuffered file
// instead of one per piece.
func (l *Logger) formatLog(log *Log, cfg *printConfig) {
	// The Printer serializes each of its Writes on its own, and a line is one
	// Write, so outMu is only needed for a per-level writer, which may be a
	// bytes.Buffer shared between a parent and its children.
	p, isPrinter := cfg.w.(*printer.Printer)
	if !isPrinter {
		l.outMu.Lock()
		defer l.outMu.Unlock()
	}

	if f := cfg.formatter; f != nil {
		if f.Format(cfg.w, log) {
			return
		}
	}

	var (
		title string
		meta  *LevelMetadata
	)
	if log.Level != DisableLevel {
		if m, ok := lookupLevel(log.Level); ok {
			meta = m
			title = m.Title
		}
	}

	buf := log.buf[:0]

	if !isPrinter && meta != nil {
		// The title goes into the same buffer; the Printer path keeps it
		// apart because it may need both a rich and a plain rendering.
		if cfg.rich {
			buf = printer.AppendRich(buf, title, meta.ColorCode, meta.Style...)
		} else {
			buf = append(buf, title...)
		}
	}
	if meta != nil {
		buf = append(buf, ' ')
	}

	if cfg.timeFormat != "" {
		buf = log.Time.AppendFormat(buf, cfg.timeFormat)
		buf = append(buf, ' ')
	}

	if log.ID != (uuid.UUID{}) {
		buf, _ = log.ID.AppendText(buf)
		buf = append(buf, ' ')
	}

	buf = append(buf, cfg.prefix...)
	buf = append(buf, log.Message...)

	if len(log.Fields) > 0 {
		// Sorted so that the same fields always print the same way; map order
		// would shuffle them from one line to the next.
		for _, k := range slices.Sorted(maps.Keys(log.Fields)) {
			buf = append(buf, ' ')
			buf = append(buf, k...)
			buf = append(buf, '=')
			buf = fmt.Append(buf, log.Fields[k])
		}
	}

	if cfg.newLine {
		buf = append(buf, '\n')
	}

	log.buf = buf

	if isPrinter && meta != nil {
		_, _ = p.WriteRichLine(title, meta.ColorCode, meta.Style, buf)
		return
	}

	_, _ = cfg.w.Write(buf)
}

// NopOutput disables the output.
var NopOutput = printer.NopOutput()

// SetOutput overrides the Logger's Printer's Output with another `io.Writer`.
//
// Returns itself.
func (l *Logger) SetOutput(w io.Writer) *Logger {
	l.Printer.SetOutput(w)
	return l
}

// AddOutput adds one or more `io.Writer` to the Logger's Printer.
//
// If one of the "writers" is not a terminal-based (i.e File)
// then colors will be disabled for all outputs.
//
// Returns itself.
func (l *Logger) AddOutput(writers ...io.Writer) *Logger {
	l.Printer.AddOutput(writers...)
	return l
}

// SetPrefix sets a prefix for this "l" Logger.
//
// The prefix is the text that is being presented
// to the output right before the log's message.
//
// Returns itself.
func (l *Logger) SetPrefix(s string) *Logger {
	l.mu.Lock()
	l.Prefix = s
	l.mu.Unlock()
	return l
}

// SetTimeFormat sets time format for logs,
// if "s" is empty then time representation will be off.
//
// Returns itself.
func (l *Logger) SetTimeFormat(s string) *Logger {
	l.mu.Lock()
	l.TimeFormat = s
	l.mu.Unlock()

	return l
}

// SetStacktraceLimit sets a stacktrace entries limit
// on `Debug` level.
// Zero means all number of stack entries will be logged.
// Negative value disables the stacktrace field.
func (l *Logger) SetStacktraceLimit(limit int) *Logger {
	l.mu.Lock()
	l.StacktraceLimit = limit
	l.mu.Unlock()

	return l
}

// SetIDGenerator sets the function that produces each Log's ID.
//
// It is nil by default, which leaves `Log.ID` the zero UUID and keeps "id" out
// of both the JSON and the plain output.
//
//	logger.SetIDGenerator(uuid.NewV7)
//
// `uuid.NewV7` is the useful default for logs: v7 UUIDs carry a timestamp in
// their leading bits, so ids sort in the order the lines were written.
//
// Returning a fixed value instead tags every entry of a logger with the same
// correlation id, which is the shape a per-request logger wants. Derive that
// logger with `Clone`, which nothing else shares:
//
//	reqID := uuid.NewV7()
//	reqLogger := logger.Clone().SetIDGenerator(func() uuid.UUID { return reqID })
//
// `Child` is not the tool for this: it caches one logger per key, so two
// requests on the same route would be setting each other's id.
//
// Pass nil to turn ids back off.
//
// Returns itself.
func (l *Logger) SetIDGenerator(newID func() uuid.UUID) *Logger {
	l.mu.Lock()
	l.NewID = newID
	l.mu.Unlock()

	return l
}

// DisableNewLine disables the new line suffix on every log function, even the `F`'s,
// the caller should add "\n" to the log message manually after this call.
//
// Returns itself.
func (l *Logger) DisableNewLine() *Logger {
	l.mu.Lock()
	l.NewLine = false
	l.mu.Unlock()

	return l
}

// RegisterFormatter registers a Formatter for this logger.
func (l *Logger) RegisterFormatter(f Formatter) *Logger {
	l.mu.Lock()
	l.formatters[f.String()] = f
	l.mu.Unlock()
	return l
}

// SetFormat sets a formatter for all logger's logs.
func (l *Logger) SetFormat(formatter string, opts ...any) *Logger {
	l.mu.Lock()
	if f, ok := l.formatters[formatter]; ok {
		l.formatter = f.Options(opts...)
	}
	l.mu.Unlock()

	return l
}

// SetLevelFormat changes the output format for the given "levelName".
func (l *Logger) SetLevelFormat(levelName string, formatter string, opts ...any) *Logger {
	level := ParseLevel(levelName)

	l.mu.Lock()
	if f, ok := l.formatters[formatter]; ok {
		l.LevelFormatter[level] = f.Options(opts...)
	}
	l.mu.Unlock()

	return l
}

// getFormatter returns the formatter responsible for a log at "level".
//
// The lookup keys on the level of the entry being written, not on the logger's
// own threshold: SetLevelFormat("info", "json") is a statement about info logs,
// and keying on l.Level made it apply to every level whenever the threshold
// happened to sit at info, and to none when it sat anywhere else.
//
// The caller holds mu.
func (l *Logger) getFormatter(level Level) Formatter {
	if f, ok := l.LevelFormatter[level]; ok {
		return f
	}

	return l.formatter
}

// SetLevelOutput sets a destination log output for the specific "levelName".
// For multiple writers use the `io.Multiwriter` wrapper.
//
// A writer shared between unrelated loggers must be safe for concurrent use.
// A parent and its children already take turns with each other.
func (l *Logger) SetLevelOutput(levelName string, w io.Writer) *Logger {
	level := ParseLevel(levelName)
	// Asked once, here, rather than on every line: for an *os.File the
	// answer is a syscall.
	rich := printer.SupportsColor(w)

	l.mu.Lock()
	l.LevelOutput[level] = w
	l.levelRich[level] = rich
	l.mu.Unlock()
	return l
}

// GetLevelOutput returns the responsible writer for the given "levelName".
// If not a registered writer is set for that level then it returns
// the logger's default printer. It does NOT return nil.
func (l *Logger) GetLevelOutput(levelName string) io.Writer {
	level := ParseLevel(levelName)

	l.mu.RLock()
	w, ok := l.LevelOutput[level]
	if !ok {
		w = l.Printer
	}
	l.mu.RUnlock()
	return w
}

// SetLevel accepts a string representation of
// a `Level` and returns a `Level` value based on that "levelName".
//
// Available level names are:
// "disable"
// "fatal"
// "error"
// "warn"
// "info"
// "debug"
//
// Alternatively you can use the exported `Level` field, i.e `Level = golog.ErrorLevel`
//
// Returns itself.
func (l *Logger) SetLevel(levelName string) *Logger {
	l.mu.Lock()
	l.Level = ParseLevel(levelName)
	l.mu.Unlock()

	return l
}

// begin is the one read-locked step of a log line: it reports whether a log at
// "level" is enabled and, if so, copies the configuration the line needs. The
// callers format their message only after a true answer, so a disabled level
// costs one lock and nothing else.
func (l *Logger) begin(level Level) (cfg printConfig, enabled bool) {
	l.mu.RLock()
	if enabled = l.Level >= level; enabled {
		cfg = l.snapshot(level)
	}
	l.mu.RUnlock()
	return cfg, enabled
}

// emit writes one log line under the configuration begin returned. "println"
// is true only for the Println family, so that a Handler can tell those apart
// from Print; every other method passes false and gets the logger's NewLine
// setting.
func (l *Logger) emit(level Level, msg string, println bool, fields Fields, cfg *printConfig) {
	log := l.acquireLog(level, msg, println || cfg.newLine, fields, cfg)
	if level == DebugLevel {
		log.Stacktrace = appendStacktrace(log.Stacktrace, cfg.stacktraceLimit)
	}
	// if not handled by one of the handler
	// then format and print it as usual.
	if !handled(cfg.handlers, log) {
		l.formatLog(log, cfg)
	}

	l.releaseLog(log)
}

// print is begin and emit for a message that is already formatted.
func (l *Logger) print(level Level, msg string, println bool, fields Fields) {
	if cfg, ok := l.begin(level); ok {
		l.emit(level, msg, println, fields, &cfg)
	}
}

// Print prints a log message without levels and colors.
func (l *Logger) Print(v ...any) {
	l.print(DisableLevel, fmt.Sprint(v...), false, nil)
}

// Printf formats according to a format specifier and writes to the output without levels and colors.
func (l *Logger) Printf(format string, args ...any) {
	l.print(DisableLevel, fmt.Sprintf(format, args...), false, nil)
}

// Println prints a log message without levels and colors.
// It adds a new line at the end, it overrides the `NewLine` option.
func (l *Logger) Println(v ...any) {
	l.print(DisableLevel, fmt.Sprint(v...), true, nil)
}

// splitArgsFields splits the given values to arguments and fields.
// It returns the arguments and the fields.
// It's used to separate the arguments from the fields
// when a `Fields` or `[]slog.Attr` is passed as a value.
func splitArgsFields(values []any) ([]any, Fields) {
	var (
		args   = values[:0]
		fields Fields
	)

	for _, value := range values {
		switch f := value.(type) {
		case Fields:
			if fields == nil {
				fields = make(Fields)
			}

			maps.Copy(fields, f)
		case []slog.Attr:
			if fields == nil {
				fields = make(Fields)
			}

			for _, attr := range f {
				fields[attr.Key] = attr.Value.Any()
			}
		case slog.Attr: // a single slog attr.
			if fields == nil {
				fields = make(Fields)
			}

			fields[f.Key] = f.Value.Any()
		default:
			args = append(args, value) // use it as fmt argument.
		}
	}

	return args, fields
}

// Log prints a leveled log message to the output.
// This method can be used to use custom log levels if needed.
// It adds a new line in the end.
func (l *Logger) Log(level Level, v ...any) {
	if cfg, ok := l.begin(level); ok {
		args, fields := splitArgsFields(v)
		l.emit(level, fmt.Sprint(args...), false, fields, &cfg)
	}
}

// Logf prints a leveled log message to the output.
// This method can be used to use custom log levels if needed.
// It adds a new line in the end.
func (l *Logger) Logf(level Level, format string, args ...any) {
	if cfg, ok := l.begin(level); ok {
		arguments, fields := splitArgsFields(args)
		l.emit(level, sprintf(format, arguments), false, fields, &cfg)
	}
}

// sprintf formats only when there is something to format with, so that a
// message passed on its own keeps its percent signs.
func sprintf(format string, args []any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// Fatal `os.Exit(1)` exit no matter the level of the logger.
// If the logger's level is fatal, error, warn, info or debug
// then it will print the log message too.
func (l *Logger) Fatal(v ...any) {
	// Not through Log: a logger set to "disable" skips the line but must
	// still exit, which is why the exit sits here and not behind the gate.
	if cfg, ok := l.begin(FatalLevel); ok {
		args, fields := splitArgsFields(v)
		l.emit(FatalLevel, fmt.Sprint(args...), false, fields, &cfg)
	}
	os.Exit(1)
}

// Fatalf will `os.Exit(1)` no matter the level of the logger.
// If the logger's level is fatal, error, warn, info or debug
// then it will print the log message too.
func (l *Logger) Fatalf(format string, args ...any) {
	if cfg, ok := l.begin(FatalLevel); ok {
		arguments, fields := splitArgsFields(args)
		l.emit(FatalLevel, sprintf(format, arguments), false, fields, &cfg)
	}
	os.Exit(1)
}

// Error will print only when logger's Level is error, warn, info or debug.
func (l *Logger) Error(v ...any) {
	l.Log(ErrorLevel, v...)
}

// Errorf will print only when logger's Level is error, warn, info or debug.
func (l *Logger) Errorf(format string, args ...any) {
	l.Logf(ErrorLevel, format, args...)
}

// Warn will print when logger's Level is warn, info or debug.
func (l *Logger) Warn(v ...any) {
	l.Log(WarnLevel, v...)
}

// Warnf will print when logger's Level is warn, info or debug.
func (l *Logger) Warnf(format string, args ...any) {
	l.Logf(WarnLevel, format, args...)
}

// Warningf exactly the same as `Warnf`.
// It's here for badger integration:
// https://github.com/dgraph-io/badger/blob/ef28ef36b5923f12ffe3a1702bdfa6b479db6637/logger.go#L25
func (l *Logger) Warningf(format string, args ...any) {
	l.Warnf(format, args...)
}

// Info will print when logger's Level is info or debug.
func (l *Logger) Info(v ...any) {
	l.Log(InfoLevel, v...)
}

// Infof will print when logger's Level is info or debug.
func (l *Logger) Infof(format string, args ...any) {
	l.Logf(InfoLevel, format, args...)
}

// Debug will print when logger's Level is debug.
func (l *Logger) Debug(v ...any) {
	l.Log(DebugLevel, v...)
}

// Debugf will print when logger's Level is debug.
func (l *Logger) Debugf(format string, args ...any) {
	l.Logf(DebugLevel, format, args...)
}

// Install receives  an external logger
// and automatically adapts its print functions.
//
// Install adds a golog handler to support third-party integrations,
// it can be used only once per `golog#Logger` instance.
//
// For example, if you want to print using a logrus
// logger you can do the following:
//
//	Install(logrus.StandardLogger())
//
// Or the standard log's Logger:
//
//	import "log"
//	myLogger := log.New(os.Stdout, "", 0)
//	Install(myLogger)
//
// Or even the slog/log's Logger:
//
//	import "log/slog"
//	myLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	Install(myLogger) OR Install(slog.Default())
//
// Look `golog#Logger.Handle` for more.
func (l *Logger) Install(logger any) {
	l.Handle(integrate(logger))
}

// Handle adds a log handler.
//
// Handlers can be used to intercept the message between a log value
// and the actual print operation, it's called
// when one of the print functions called.
// If it's return value is true then it means that the specific
// handler handled the log by itself therefore no need to
// proceed with the default behavior of printing the log
// to the specified logger's output.
//
// It stops on the handler which returns true firstly.
// The `Log` value holds the level of the print operation as well.
func (l *Logger) Handle(handler Handler) {
	l.mu.Lock()
	// Copy on write: a print in flight may still be walking the old slice,
	// and a Clone may share its backing array. Neither must see this append.
	l.handlers = append(slices.Clip(l.handlers), handler)
	l.mu.Unlock()
}

func handled(handlers []Handler, value *Log) bool {
	for _, h := range handlers {
		if h(value) {
			return true
		}
	}
	return false
}

// Scan reads "r" line by line and prints each line through this logger the
// way `Println` would: with the time prefix and the logger's prefix, past its
// handlers and formatters, but without a level.
//
// It runs until "r" is exhausted or the returned "cancel" is called, whichever
// comes first; "cancel" is safe to call more than once. A read error other than
// the end of the input is logged at the error level. One line is capped at 1MB:
// the rest of a longer line is dropped and the kept part is marked as truncated.
func (l *Logger) Scan(r io.Reader) (cancel func()) {
	stop := make(chan struct{})

	go printer.ScanLines(r, stop,
		func(line []byte) { l.print(DisableLevel, string(line), true, nil) },
		func(err error) { l.Errorf("scan: %v", err) },
	)

	// Callers defer cancel and then call it early on an error path, and that
	// has to be safe rather than fatal.
	return sync.OnceFunc(func() { close(stop) })
}

// Clone returns a copy of this "l" Logger.
// This copy is returned as pointer as well.
//
// The copy shares nothing mutable with the original except the writers
// themselves and the write lock that keeps the two from interleaving lines.
func (l *Logger) Clone() *Logger {
	// Read under the lock: RegisterFormatter, SetLevelFormat and SetLevelOutput
	// all write these maps, and Child clones on demand from whichever goroutine
	// asked for the child.
	l.mu.RLock()
	defer l.mu.RUnlock()

	return &Logger{
		Prefix:          l.Prefix,
		Level:           l.Level,
		TimeFormat:      l.TimeFormat,
		StacktraceLimit: l.StacktraceLimit,
		NewLine:         l.NewLine,
		NewID:           l.NewID,
		outMu:           l.outMu,
		Printer:         l.Printer.Clone(),
		LevelOutput:     maps.Clone(l.LevelOutput),
		levelRich:       maps.Clone(l.levelRich),
		formatter:       l.formatter,
		formatters:      maps.Clone(l.formatters),
		LevelFormatter:  maps.Clone(l.LevelFormatter),
		handlers:        slices.Clone(l.handlers),
		children:        newLoggerMap(),
	}
}

// Child (creates if not exists and) returns a new child
// Logger based on the current logger's fields.
//
// Can be used to separate logs by category.
// If the "key" is string then it's used as prefix,
// which is appended to the current prefix one.
func (l *Logger) Child(key any) *Logger {
	return l.children.getOrAdd(key, l)
}

// SetChildPrefix same as `SetPrefix` but it does NOT
// override the existing, instead the given "prefix"
// is appended to the current one. It's useful
// to chian loggers with their own names/prefixes.
// It does add the ": " in the end of "prefix" if it's missing.
// It returns itself.
func (l *Logger) SetChildPrefix(prefix string) *Logger {
	if prefix == "" {
		return l
	}

	// if prefix doesn't end with a whitespace, then add it here.
	if !strings.HasSuffix(prefix, ": ") {
		prefix += ": "
	}

	l.mu.Lock()
	if l.Prefix != "" {
		if !strings.HasSuffix(l.Prefix, " ") {
			l.Prefix += " "
		}
	}
	l.Prefix += prefix
	l.mu.Unlock()

	return l
}

// LastChild returns the last registered child Logger.
func (l *Logger) LastChild() *Logger {
	return l.children.getLast()
}

// RemoveChild removes a child logger by its key.
// Returns true if the child was found and removed, false otherwise.
func (l *Logger) RemoveChild(key any) bool {
	return l.children.remove(key)
}

// ClearChildren removes all child loggers.
func (l *Logger) ClearChildren() {
	l.children.clear()
}

// ChildCount returns the number of child loggers.
func (l *Logger) ChildCount() int {
	return l.children.count()
}

// ListChildKeys returns a slice of all child logger keys.
func (l *Logger) ListChildKeys() []any {
	return l.children.listKeys()
}

type loggerMap struct {
	mu    sync.RWMutex
	Items map[any]*Logger
	// keys in registration order.
	//
	// This used to be a map[int]any, and remove() rebuilt it by ranging over
	// it -- over a Go map, whose iteration order is deliberately randomized.
	// So the one operation that existed to preserve the order was the one that
	// scrambled it, and getLast() read the result.
	itemsOrdered []any
}

func newLoggerMap() *loggerMap {
	return &loggerMap{
		Items: make(map[any]*Logger),
	}
}

func (m *loggerMap) getLast() *Logger {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.itemsOrdered) == 0 {
		return nil
	}

	return m.Items[m.itemsOrdered[len(m.itemsOrdered)-1]]
}

func (m *loggerMap) getOrAdd(key any, parent *Logger) *Logger {
	m.mu.RLock()
	logger, ok := m.Items[key]
	m.mu.RUnlock()
	if ok {
		return logger
	}

	logger = parent.Clone()
	childPrefix := ""
	switch v := key.(type) {
	case string:
		childPrefix = v
	case fmt.Stringer:
		childPrefix = v.String()
	}
	logger.SetChildPrefix(childPrefix)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check: two goroutines can both miss the read above and arrive here,
	// and the second must not append the key a second time.
	if existing, ok := m.Items[key]; ok {
		return existing
	}

	m.itemsOrdered = append(m.itemsOrdered, key)
	m.Items[key] = logger

	return logger
}

// remove removes a logger by its key and returns true if found and removed.
func (m *loggerMap) remove(key any) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Items[key]; !exists {
		return false
	}

	delete(m.Items, key)

	if i := slices.Index(m.itemsOrdered, key); i >= 0 {
		m.itemsOrdered = slices.Delete(m.itemsOrdered, i, i+1)
	}

	return true
}

// clear removes all child loggers.
func (m *loggerMap) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	clear(m.Items)
	m.itemsOrdered = m.itemsOrdered[:0]
}

// count returns the number of child loggers.
func (m *loggerMap) count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.Items)
}

// listKeys returns a slice of all logger keys, in registration order.
func (m *loggerMap) listKeys() []any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return slices.Clone(m.itemsOrdered)
}

// The Scan limits, mirrored from the printer package for this package's
// tests and docs.
const (
	maxScanLine   = printer.MaxScanLine
	scanTruncated = printer.ScanTruncated
)
