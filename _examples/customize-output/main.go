package main

import (
	"fmt"
	"runtime"

	"github.com/kataras/golog"
)

func main() {

	golog.Handle(func(l *golog.Log) bool {
		prefix := golog.GetTextForLevel(l.Level, true)
		pc, fn, line, _ := runtime.Caller(7)
		message := fmt.Sprintf("%s line %d (%s) (%s) %s: %s",
			prefix, line, runtime.FuncForPC(pc).Name(), fn, l.FormatTime(), l.Message)

		if l.NewLine {
			message += "\n"
		}

		fmt.Print(message)
		return true
	})

	golog.Warnf("Hey, warning here")

	golog.Errorf("Something went wrong!")
}
