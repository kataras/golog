package golog

import (
	"context"
	"log/slog"
)

func integrade(logger any) Handler {
	switch v := logger.(type) {
	case *slog.Logger:
		return integradeSlog(v)
	case ExternalLogger:
		return integrateExternalLogger(v)
	case StdLogger:
		return integrateStdLogger(v)
	default:
		panic("not supported logger integration, please open a feature request at: https://github.com/kataras/golog/issues/new")
	}
}

/*
func (*slog.Logger).Debug(msg string, args ...any)
func (*slog.Logger).DebugContext(ctx context.Context, msg string, args ...any)
func (*slog.Logger).Enabled(ctx context.Context, level slog.Level) bool
func (*slog.Logger).Error(msg string, args ...any)
func (*slog.Logger).ErrorContext(ctx context.Context, msg string, args ...any)
func (*slog.Logger).Handler() slog.Handler
func (*slog.Logger).Info(msg string, args ...any)
func (*slog.Logger).InfoContext(ctx context.Context, msg string, args ...any)
func (*slog.Logger).Log(ctx context.Context, level slog.Level, msg string, args ...any)
func (*slog.Logger).LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr)
func (*slog.Logger).Warn(msg string, args ...any)
func (*slog.Logger).WarnContext(ctx context.Context, msg string, args ...any)
func (*slog.Logger).With(args ...any) *slog.Logger
func (*slog.Logger).WithGroup(name string) *slog.Logger
*/
func integradeSlog(logger *slog.Logger) Handler {
	return func(log *Log) bool {
		// golog level to slog level.
		level := getSlogLevel(log.Level)
		// golog fields to slog attributes.
		if len(log.Fields) > 0 {
			attrs := make([]slog.Attr, 0, len(log.Fields))
			for k, v := range log.Fields {
				attrs = append(attrs, slog.Any(k, v))
			}
			// log the message with attrs.
			logger.LogAttrs(context.Background(), level, log.Message, attrs...)
		} else {
			logger.Log(context.Background(), level, log.Message)
		}

		return true
	}
}

// ExternalLogger is a typical logger interface.
// Any logger or printer that completes this interface
// can be used to intercept and handle the golog's messages.
//
// See `Logger#Install` and `Logger#Handle` for more.
type ExternalLogger interface {
	Print(...any)
	Println(...any)
	Error(...any)
	Warn(...any)
	Info(...any)
	Debug(...any)
}

// integrateExternalLogger is a Handler which
// intercepts all messages from print functions,
// between print action and actual write to the output,
// and sends these (messages) to the external "logger".
//
// In short terms, when this handler is passed via `Handle`
// then, instead of printing from the logger's Printer
// it prints from the given "logger".
func integrateExternalLogger(logger ExternalLogger) Handler {
	return func(log *Log) bool {
		printFunc := getExternalPrintFunc(logger, log)
		printFunc(log.Message)
		return true
	}
}

func getSlogLevel(level Level) slog.Level {
	switch level {
	case ErrorLevel:
		return slog.LevelError
	case WarnLevel:
		return slog.LevelWarn
	case InfoLevel:
		return slog.LevelInfo
	case DebugLevel:
		return slog.LevelDebug
	}
	return slog.LevelDebug
}

func getExternalPrintFunc(logger ExternalLogger, log *Log) func(...any) {
	switch log.Level {
	case ErrorLevel:
		return logger.Error
	case WarnLevel:
		return logger.Warn
	case InfoLevel:
		return logger.Info
	case DebugLevel:
		return logger.Debug
	}

	// disable level or use of golog#Print/Println functions:

	// passed with Println
	if log.NewLine {
		return logger.Println
	}

	return logger.Print
}

// StdLogger is the standard log.Logger interface.
// Any logger or printer that completes this interface
// can be used to intercept and handle the golog's messages.
//
// See `Logger#Install` and `Logger#Handle` for more.
type StdLogger interface {
	Printf(format string, v ...any)
	Print(v ...any)
	Println(v ...any)
}

func integrateStdLogger(logger StdLogger) Handler {
	return func(log *Log) bool {
		printFunc := getStdPrintFunc(logger, log)
		printFunc(log.Message)
		return true
	}
}

func getStdPrintFunc(logger StdLogger, log *Log) func(...any) {
	// no levels here

	// passed with Println
	if log.NewLine {
		return logger.Println
	}

	return logger.Print
}
