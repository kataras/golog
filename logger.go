package golog

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"strings"
	"sync"

	"github.com/kataras/golog/internal"
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
	mu      sync.RWMutex // for logger field changes and printing.
	Printer *internal.Printer
	// The per log level raw writers, optionally.
	LevelOutput map[Level]io.Writer

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
		Printer:     internal.NewPrinter(os.Stdout),
		LevelOutput: make(map[Level]io.Writer),
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

// acquireLog returns a new log fom the pool.
func (l *Logger) acquireLog(level Level, msg string, withPrintln bool, fields Fields) *Log {
	log, ok := l.logs.Get().(*Log)
	if !ok {
		log = &Log{
			Logger: l,
		}
	}

	log.NewLine = withPrintln
	if l.TimeFormat != "" {
		log.Time = Now()
		log.Timestamp = log.Time.Unix()
	}
	log.Level = level
	log.Message = msg
	log.Fields = fields
	log.Stacktrace = log.Stacktrace[:0]
	return log
}

// releaseLog Log releases a log instance back to the pool.
func (l *Logger) releaseLog(log *Log) {
	l.logs.Put(log)
}

var spaceBytes = []byte(" ")

// formatLog formats and writes the log entry directly to the output writer.
func (l *Logger) formatLog(log *Log) {
	l.mu.Lock()
	defer l.mu.Unlock()

	w := l.getOutput(log.Level)

	// Check if a custom formatter should handle this
	if f := l.getFormatter(); f != nil {
		if f.Format(w, log) {
			return
		}
	}

	// Format the log entry directly
	if log.Level != DisableLevel {
		if level, ok := Levels[log.Level]; ok {
			internal.WriteRich(w, level.Title, level.ColorCode, level.Style...)
			_, _ = w.Write(spaceBytes)
		}
	}

	if t := log.FormatTime(); t != "" {
		_, _ = fmt.Fprint(w, t)
		_, _ = w.Write(spaceBytes)
	}

	if prefix := l.Prefix; len(prefix) > 0 {
		_, _ = fmt.Fprint(w, prefix)
	}

	_, _ = fmt.Fprint(w, log.Message)

	for k, v := range log.Fields {
		_, _ = fmt.Fprintf(w, " %s=%v", k, v)
	}

	if l.NewLine {
		_, _ = fmt.Fprintln(w)
	}
}

// NopOutput disables the output.
var NopOutput = internal.NopOutput()

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
	l.mu.RLock()
	f, ok := l.formatters[formatter]
	l.mu.RUnlock()

	if ok {
		l.mu.Lock()
		l.formatter = f.Options(opts...)
		l.mu.Unlock()
	}

	return l
}

// SetLevelFormat changes the output format for the given "levelName".
func (l *Logger) SetLevelFormat(levelName string, formatter string, opts ...any) *Logger {
	l.mu.RLock()
	f, ok := l.formatters[formatter]
	l.mu.RUnlock()

	if ok {
		l.mu.Lock()
		l.LevelFormatter[ParseLevel(levelName)] = f.Options(opts...)
		l.mu.Unlock()
	}

	return l
}

func (l *Logger) getFormatter() Formatter {
	f, ok := l.LevelFormatter[l.Level]
	if !ok {
		f = l.formatter
	}

	if f == nil {
		return nil
	}

	return f
}

// SetLevelOutput sets a destination log output for the specific "levelName".
// For multiple writers use the `io.Multiwriter` wrapper.
func (l *Logger) SetLevelOutput(levelName string, w io.Writer) *Logger {
	l.mu.Lock()
	l.LevelOutput[ParseLevel(levelName)] = w
	l.mu.Unlock()
	return l
}

// GetLevelOutput returns the responsible writer for the given "levelName".
// If not a registered writer is set for that level then it returns
// the logger's default printer. It does NOT return nil.
func (l *Logger) GetLevelOutput(levelName string) io.Writer {
	l.mu.RLock()
	w := l.getOutput(ParseLevel(levelName))
	l.mu.RUnlock()
	return w
}

func (l *Logger) getOutput(level Level) io.Writer {
	w, ok := l.LevelOutput[level]
	if !ok {
		w = l.Printer
	}
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

func (l *Logger) print(level Level, msg string, newLine bool, fields Fields) {
	if l.Level >= level {
		// newLine passed here in order for handler to know
		// if this message derives from Println and Leveled functions
		// or by simply, Print.
		log := l.acquireLog(level, msg, newLine, fields)
		if level == DebugLevel {
			log.Stacktrace = GetStacktrace(l.StacktraceLimit)
		}
		// if not handled by one of the handler
		// then format and print it as usual.
		if !l.handled(log) {
			l.formatLog(log)
		}

		l.releaseLog(log)
	}
	// if level was fatal we don't care about the logger's level, we'll exit.
	if level == FatalLevel {
		os.Exit(1)
	}
}

// Print prints a log message without levels and colors.
func (l *Logger) Print(v ...any) {
	l.print(DisableLevel, fmt.Sprint(v...), l.NewLine, nil)
}

// Printf formats according to a format specifier and writes to `Printer#Output` without levels and colors.
func (l *Logger) Printf(format string, args ...any) {
	l.print(DisableLevel, fmt.Sprintf(format, args...), l.NewLine, nil)
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

			for k, v := range f {
				fields[k] = v
			}
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
	if l.Level >= level {
		args, fields := splitArgsFields(v)
		l.print(level, fmt.Sprint(args...), l.NewLine, fields)
	}
}

// Logf prints a leveled log message to the output.
// This method can be used to use custom log levels if needed.
// It adds a new line in the end.
func (l *Logger) Logf(level Level, format string, args ...any) {
	if l.Level >= level {
		arguments, fields := splitArgsFields(args)
		msg := format
		if len(arguments) > 0 {
			msg = fmt.Sprintf(msg, arguments...)
		}
		l.print(level, msg, l.NewLine, fields)
	}
}

// Fatal `os.Exit(1)` exit no matter the level of the logger.
// If the logger's level is fatal, error, warn, info or debug
// then it will print the log message too.
func (l *Logger) Fatal(v ...any) {
	l.Log(FatalLevel, v...)
}

// Fatalf will `os.Exit(1)` no matter the level of the logger.
// If the logger's level is fatal, error, warn, info or debug
// then it will print the log message too.
func (l *Logger) Fatalf(format string, args ...any) {
	l.Logf(FatalLevel, format, args...)
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
	l.handlers = append(l.handlers, handler)
	l.mu.Unlock()
}

func (l *Logger) handled(value *Log) (handled bool) {
	for _, h := range l.handlers {
		if h(value) {
			return true
		}
	}
	return false
}

// Scan scans everything from "r" and prints
// its new contents to the logger's Printer's Output,
// forever or until the returning "cancel" is fired, once.
func (l *Logger) Scan(r io.Reader) (cancel func()) {
	// Create a custom scanner that adds time formatting
	scanner := &timeScanner{
		logger: l,
		reader: r,
	}
	return scanner.scan()
}

// timeScanner handles scanning with time formatting
type timeScanner struct {
	logger *Logger
	reader io.Reader
}

func (ts *timeScanner) scan() (cancel func()) {
	stop := make(chan struct{})

	go func() {
		defer close(stop)

		// Use bufio.Scanner to read lines
		scanner := bufio.NewScanner(ts.reader)
		for scanner.Scan() {
			select {
			case <-stop:
				return
			default:
				line := scanner.Bytes()
				if len(line) > 0 {
					// Add time prefix if TimeFormat is set
					var formattedLine []byte
					if ts.logger.TimeFormat != "" {
						timePrefix := Now().Format(ts.logger.TimeFormat) + " "
						formattedLine = append([]byte(timePrefix), line...)
					} else {
						formattedLine = make([]byte, len(line))
						copy(formattedLine, line)
					}

					// Write the formatted line with newline
					data := append(formattedLine, '\n')
					ts.logger.Printer.Write(data)
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

// Clone returns a copy of this "l" Logger.
// This copy is returned as pointer as well.
func (l *Logger) Clone() *Logger {
	// copy level output and format maps.
	formats := make(map[string]Formatter, len(l.formatters))
	maps.Copy(formats, l.formatters)

	levelFormat := make(map[Level]Formatter, len(l.LevelFormatter))
	maps.Copy(levelFormat, l.LevelFormatter)

	levelOutput := make(map[Level]io.Writer, len(l.LevelOutput))
	maps.Copy(levelOutput, l.LevelOutput)

	return &Logger{
		Prefix:         l.Prefix,
		Level:          l.Level,
		TimeFormat:     l.TimeFormat,
		NewLine:        l.NewLine,
		Printer:        l.Printer,
		LevelOutput:    levelOutput,
		formatter:      l.formatter,
		formatters:     formats,
		LevelFormatter: levelFormat,
		handlers:       l.handlers,
		children:       newLoggerMap(),
		mu:             sync.RWMutex{},
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
	mu           sync.RWMutex
	Items        map[any]*Logger
	itemsOrdered map[int]any // registration order of logger and its key.
}

func newLoggerMap() *loggerMap {
	return &loggerMap{
		Items:        make(map[any]*Logger),
		itemsOrdered: make(map[int]any),
	}
}

func (m *loggerMap) getByIndex(index int) (l *Logger) {
	m.mu.RLock()
	if key, ok := m.itemsOrdered[index]; ok {
		l = m.Items[key]
	}
	m.mu.RUnlock()

	return l
}

func (m *loggerMap) getLast() *Logger {
	m.mu.RLock()
	n := len(m.Items)
	m.mu.RUnlock()
	if n == 0 {
		return nil
	}

	return m.getByIndex(n - 1)
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
	m.itemsOrdered[len(m.Items)] = key
	m.Items[key] = logger
	m.mu.Unlock()

	return logger
}

// remove removes a logger by its key and returns true if found and removed.
func (m *loggerMap) remove(key any) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.Items[key]
	if !exists {
		return false
	}

	delete(m.Items, key)

	// Rebuild the ordered map
	newOrdered := make(map[int]any)
	index := 0
	for _, v := range m.itemsOrdered {
		if v != key {
			newOrdered[index] = v
			index++
		}
	}
	m.itemsOrdered = newOrdered

	return true
}

// clear removes all child loggers.
func (m *loggerMap) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Items = make(map[any]*Logger)
	m.itemsOrdered = make(map[int]any)
}

// count returns the number of child loggers.
func (m *loggerMap) count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.Items)
}

// listKeys returns a slice of all logger keys.
func (m *loggerMap) listKeys() []any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]any, 0, len(m.Items))
	for key := range m.Items {
		keys = append(keys, key)
	}

	return keys
}
