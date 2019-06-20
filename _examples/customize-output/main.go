package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/kataras/golog"
)

// https://golang.org/doc/go1.9#callersframes
func getCaller() (string, int) {
	var pcs [10]uintptr
	n := runtime.Callers(1, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()

		if !strings.HasSuffix(frame.File, "github.com/kataras/golog") && frame.Func.Name() != "main.getCaller" {
			return frame.File, frame.Line
		}

		if !more {
			break
		}
	}

	return "?", 0
}

func main() {

	golog.Handle(func(l *golog.Log) bool {
		prefix := golog.GetTextForLevel(l.Level, true)

		filename, line := getCaller()
		message := fmt.Sprintf("%s %s [%s:%d] %s",
			prefix, l.FormatTime(), filename, line, l.Message)

		if l.NewLine {
			message += "\n"
		}

		fmt.Print(message)
		return true
	})

	golog.Warnf("Hey, warning here")

	golog.Errorf("Something went wrong!")
}
