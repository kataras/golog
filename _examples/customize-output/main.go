package main

import (
	"encoding/json"
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
		funcName := frame.Func.Name()

		if (!strings.Contains(frame.File, "github.com/kataras/golog") || strings.Contains(frame.File, "_examples")) &&
			funcName != "main.getCaller" &&
			funcName != "main.simpleOutput" &&
			funcName != "main.jsonOutput1" &&
			funcName != "main.jsonOutput2" {
			return frame.File, frame.Line
		}

		if !more {
			break
		}
	}

	return "?", 0
}

// Some format examples...

func simpleOutput(l *golog.Log) bool {
	prefix := golog.GetTextForLevel(l.Level, true)

	filename, line := getCaller()
	message := fmt.Sprintf("%s %s [%s:%d] %s",
		prefix, l.FormatTime(), filename, line, l.Message)

	if l.NewLine {
		message += "\n"
	}

	fmt.Print(message)
	return true
}

func jsonOutput1(l *golog.Log) bool {
	source, line := getCaller()
	b, _ := json.MarshalIndent(struct {
		Datetime string `json:"datetime"`
		Level    string `json:"level"`
		Message  string `json:"message"`
		Source   string `json:"source"`
	}{
		l.FormatTime(),
		golog.GetTextForLevel(l.Level, false),
		l.Message,
		fmt.Sprintf("%s#%d", source, line),
	}, "", "    ")

	fmt.Print(string(b))
	return true
}

func jsonOutput2(l *golog.Log) bool {
	fn, line := getCaller()

	var (
		datetime = l.FormatTime()
		level    = golog.GetTextForLevel(l.Level, false)
		message  = l.Message
		source   = fmt.Sprintf("%s#%d", fn, line)
	)
	jsonStr := fmt.Sprintf("{\n\t\"datetime\":\"%s\",\n\t\"level\":\"%s\",\n\t\"message\":\"%s\",\n\t\"source\":\"%s\"\n}", datetime, level, message, source)
	fmt.Println(jsonStr)

	return true
}

func main() {
	// golog.Handle(simpleOutput)
	golog.Handle(jsonOutput1)
	// golog.Handle(jsonOutput2)

	golog.Warnf("Hey, warning here")

	golog.Errorf("Something went wrong!")
}
