package main

import (
	"github.com/kataras/golog"
)

func main() {

	// First argument is the raw text for outputs
	// that are not support colors,
	// second argument is the full colorful text (yes it can be different if you wish to).
	//
	// If the second argument is empty then golog will update the colorful text to the
	// default color (i.e red on ErrorText) based on the first argument.

	// Default is "[ERRO]"
	golog.ErrorText("|ERROR|", "")
	// Default is "[WARN]"
	golog.WarnText("|WARN|", "")
	// Default is "[INFO]"
	golog.InfoText("|INFO|", "")
	// Default is "[DBUG]"
	golog.DebugText("|DEBUG|", "")

	// Business as usual...
	golog.SetLevel("debug")

	golog.Println("This is a raw message, no levels, no colors.")
	golog.Info("This is an info message, with colors (if the output is terminal)")
	golog.Warn("This is a warning message")
	golog.Error("This is an error message")
	golog.Debug("This is a debug message")
}
