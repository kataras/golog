package main

import (
	"log"
	"os"

	"github.com/kataras/golog"
)

// simulate a log.Logger preparation:
var myLogger = log.New(os.Stdout, "", 0)

func main() {
	golog.SetLevel("error")
	golog.InstallStd(myLogger)

	golog.Debug(`this debug message will not be shown,
	because the golog level is ErrorLevel`)

	golog.Error("this error message will be visible the only visible")

	golog.Warn("this info message will not be visible")
}
