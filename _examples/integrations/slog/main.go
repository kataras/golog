package main

import (
	"log/slog"
	"os"

	"github.com/kataras/golog"
)

// simulate an slog.Logger preparation:
var myLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

func main() {
	golog.SetLevel("error")
	golog.Install(myLogger)

	golog.Debug(`this debug message will not be shown,
	because the golog level is ErrorLevel`)

	golog.Error("this error message will be visible the only visible")

	golog.Warn("this info message will not be visible")
}
