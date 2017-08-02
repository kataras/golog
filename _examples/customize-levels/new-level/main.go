package main

import (
	"github.com/kataras/golog"
)

func main() {
	// Let's add a custom level,
	//
	// It should be starting from level index 6,
	// because we have 6 built'n levels  (0 is the start index):
	// disable,
	// fatal,
	// error,
	// warn,
	// info
	// debug

	// First we create our level to a golog.Level
	// in order to be used in the Log functions.
	var SuccessLevel golog.Level = 6
	// Register our level, just three fields.
	golog.Levels[SuccessLevel] = &golog.LevelMetadata{
		Name:    "success",
		RawText: "[SUCC]",
		// ColorfulText (Green Color[SUCC])
		ColorfulText: "\x1b[32m[SUCC]\x1b[0m",
	}

	// create a new golog logger
	myLogger := golog.New()

	// set its level to the higher in order to see it
	// ("success" is the name we gave to our level)
	myLogger.SetLevel("success")

	// and finally print a log message with our custom level
	myLogger.Logf(SuccessLevel, "This is a success log message with green color")
}
