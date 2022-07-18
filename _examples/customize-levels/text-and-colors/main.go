package main

import (
	"github.com/kataras/golog"
)

func main() {

	// First argument is the raw text for outputs
	// second argument is the color code
	// and the last, variadic argument can be any `kataras/pio.RichOption`, e.g. pio.Background, pio.Underline.

	// Default is "[ERRO]"
	golog.ErrorText("|ERROR|", 31)
	// Default is "[WARN]"
	golog.WarnText("|WARN|", 32)
	// Default is "[INFO]"
	golog.InfoText("|INFO|", 34)
	// Default is "[DBUG]"
	golog.DebugText("|DEBUG|", 33)

	// Business as usual...
	golog.SetLevel("debug")

	golog.Println("This is a raw message, no levels, no colors.")
	golog.Info("This is an info message, with colors (if the output is terminal)")
	golog.Warn("This is a warning message")
	golog.Error("This is an error message")
	golog.Debug("This is a debug message")
}
