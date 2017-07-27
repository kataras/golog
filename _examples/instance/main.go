package main

import (
	"github.com/kataras/golog"
)

func main() {
	log := golog.New()

	// Default Output is `os.Stderr`,
	// but you can change it:
	// log.SetOutput(os.Stdout)

	// Level defaults to "info",
	// but you can change it:
	log.SetLevel("debug")

	log.Println("This is a raw message, no levels, no colors.")
	log.Info("This is an info message, with colors (if the output is terminal)")
	log.Warn("This is a warning message")
	log.Error("This is an error message")
	log.Debug("This is a debug message")
}
