package golog

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/kataras/pio"
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
	Level      Level
	TimeFormat string
	mu         sync.Mutex
	Printer    *pio.Printer
	handlers   []Handler
	once       sync.Once
	logs       sync.Pool
}

// New returns a new golog, default Level is `InfoLevel`.
func New() *Logger {
	return &Logger{
		Level:      InfoLevel,
		TimeFormat: "2006/01/02 15:04",
		Printer:    pio.NewPrinter("", os.Stderr).EnableDirectOutput().Hijack(logHijacker),
	}
}

// acquireLog returns a new log fom the pool.
func (l *Logger) acquireLog(level Level, msg string, withPrintln bool) *Log {
	log, ok := l.logs.Get().(*Log)
	if !ok {
		log = &Log{
			Logger: l,
		}
	}
	log.NewLine = withPrintln
	log.Time = time.Now()
	log.Level = level
	log.Message = msg
	return log
}

// releaseLog Log releases a log instance back to the pool.
func (l *Logger) releaseLog(log *Log) {
	l.logs.Put(log)
}

// we could use marshal inside Log but we don't have access to printer,
// we could also use the .Handle with NopOutput too but
// this way is faster:
var logHijacker = func(ctx *pio.Ctx) {
	l, ok := ctx.Value.(*Log)
	if !ok {
		ctx.Next()
		return
	}

	// prefix := prefixFromLevel(l.Level, ctx.Printer.IsTerminal)
	// if prefix != "" {
	// 	prefix += " "
	// }

	// formattedTime := l.FormatTime()
	// if formattedTime != "" {
	// 	formattedTime += " "
	// }
	// // edw adi gia sprintf na to kanw me aplo string kai na ksanatreksw ta benchmarks
	// // eixe 4k ns (sto travis) tin prigoumenh fora
	// ctx.Store([]byte(fmt.Sprintf("%s%s%s", prefix, formattedTime, l.Message)), nil)
	line := prefixFromLevel(l.Level, ctx.Printer.IsTerminal)
	if line != "" {
		line += " "
	}
	if t := l.FormatTime(); t != "" {
		line += t + " "
	}
	line += l.Message
	if l.NewLine {
		line += "\n"
	}
	ctx.Store([]byte(line), nil)
	ctx.Next()
}

// NopOutput disables the output.
var NopOutput = pio.NopOutput()

// SetOutput overrides the Logger's Printer's Output with another `io.Writer`.
func (l *Logger) SetOutput(w io.Writer) {
	l.Printer.SetOutput(w)
}

// AddOutput adds one or more `io.Writer` to the Logger's Printer.
//
// If one of the "writers" is not a terminal-based (i.e File)
// then colors will be disabled for all outputs.
func (l *Logger) AddOutput(writers ...io.Writer) {
	l.Printer.AddOutput(writers...)
}

// SetTimeFormat sets time format for logs,
// if "s" is empty then time representation will be off.
func (l *Logger) SetTimeFormat(s string) {
	l.mu.Lock()
	l.TimeFormat = s
	l.mu.Unlock()
}

// SetLevel accepts a string representation of
// a `Level` and returns a `Level` value based on that "levelName".
//
// Available level names are:
// "disable"
// "error"
// "warn"
// "info"
// "debug"
//
// Alternatively you can use the exported `Level` field, i.e `Level = golog.ErrorLevel`
func (l *Logger) SetLevel(levelName string) {
	l.mu.Lock()
	l.Level = fromLevelName(levelName)
	l.mu.Unlock()
}

func (l *Logger) print(level Level, msg string, newLine bool) {
	if l.Level >= level {
		// newLine passed here in order for handler to know
		// if this message derives from Println and Leveled functions
		// or by simply, Print.
		log := l.acquireLog(level, msg, newLine)
		// if not handled by one of the handler
		// then print it as usual.
		if !l.handled(log) {
			if newLine {
				l.Printer.Println(log)
			} else {
				l.Printer.Print(log)
			}
		}

		l.releaseLog(log)
	}
}

// Print prints a log message without levels and colors.
func (l *Logger) Print(v ...interface{}) {
	l.print(DisableLevel, fmt.Sprint(v...), false)
}

// Println prints a log message without levels and colors.
// It adds a new line at the end.
func (l *Logger) Println(v ...interface{}) {
	l.print(DisableLevel, fmt.Sprint(v...), true)
}

// Error will print only when logger's Level is error.
func (l *Logger) Error(v ...interface{}) {
	l.print(ErrorLevel, fmt.Sprint(v...), true)
}

// Errorf will print only when logger's Level is error.
func (l *Logger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Error(msg)
}

// Warn will print when logger's Level is error, or warning.
func (l *Logger) Warn(v ...interface{}) {
	l.print(WarnLevel, fmt.Sprint(v...), true)
}

// Warnf will print when logger's Level is error, or warning.
func (l *Logger) Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Warn(msg)
}

// Info will print when logger's Level is error, warning or info.
func (l *Logger) Info(v ...interface{}) {
	l.print(InfoLevel, fmt.Sprint(v...), true)
}

// Infof will print when logger's Level is error, warning or info.
func (l *Logger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Info(msg)
}

// Debug will print when logger's Level is error, warning,info or debug.
func (l *Logger) Debug(v ...interface{}) {
	l.print(DebugLevel, fmt.Sprint(v...), true)
}

// Debugf will print when logger's Level is error, warning,info or debug.
func (l *Logger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Debug(msg)
}

// Install receives  an external logger
// and automatically adapts its print functions.
//
// Install adds a golog handler to support third-party integrations,
// it can be used only once per `golog#Logger` instance.
//
// For example, if you want to print using a logrus
// logger you can do the following:
// `Install(logrus.StandardLogger())`
//
// Look `golog#Logger.Handle` for more.
func (l *Logger) Install(logger ExternalLogger) {
	l.Handle(integrateExternalLogger(logger))
}

// InstallStd receives  a standard logger
// and automatically adapts its print functions.
//
// Install adds a golog handler to support third-party integrations,
// it can be used only once per `golog#Logger` instance.
//
// Example Code:
//	import "log"
//	myLogger := log.New(os.Stdout, "", 0)
//	InstallStd(myLogger)
//
// Look `golog#Logger.Handle` for more.
func (l *Logger) InstallStd(logger StdLogger) {
	l.Handle(integrateStdLogger(logger))
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

// Hijack adds a hijacker to the low-level logger's Printer.
// If you need to implement such as a low-level hijacker manually,
// then you have to make use of the pio library.
func (l *Logger) Hijack(hijacker func(ctx *pio.Ctx)) {
	l.Printer.Hijack(hijacker)
}

// Scan scans everything from "r" and prints
// its new contents to the logger's Printer's Output,
// forever or until the returning "cancel" is fired, once.
func (l *Logger) Scan(r io.Reader) (cancel func()) {
	l.once.Do(func() {
		// add a marshaler once
		// in order to handle []byte and string
		// as its input.
		// Because scan doesn't care about
		// logging levels (because of the io.Reader)
		// Note: We don't use the `pio.Text` built'n marshaler
		// because we want to manage time log too.
		l.Printer.MarshalFunc(func(v interface{}) ([]byte, error) {
			var line []byte
			if b, ok := v.([]byte); ok {
				line = b
			} else if s, ok := v.(string); ok {
				line = []byte(s)
			}

			if len(line) == 0 {
				return nil, pio.ErrMarshalNotResponsible
			}

			formattedTime := time.Now().Format(l.TimeFormat)
			if formattedTime != "" {
				line = append([]byte(formattedTime+" "), line...)
			}

			return line, nil
		})
	})

	return l.Printer.Scan(r, true)
}
