// Package main shows how you can register a log output per level.
package main

import (
	"os"

	"github.com/kataras/golog"
)

func simple() {
	golog.SetLevelOutput("error", os.Stderr)

	golog.Error("an error") // prints to os.Stderr.
	golog.Info("an info")   // prints to the default output: os.Stdout.
}

func main() {
	// use  debug.log and info.log files for the example.
	debugFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer debugFile.Close()

	infoFile, err := os.OpenFile("info.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer infoFile.Close()
	//

	// initialize a new logger
	logger := golog.New()
	logger.SetLevel("debug")
	//

	// set the outputs per log level.
	logger.SetLevelOutput("debug", debugFile)
	logger.SetLevelOutput("info", infoFile)
	//

	// write some logs.
	logger.Debug("A debug message")
	// debug.log contains:
	// [DBUG] 2020/09/06 12:01 A debug message
	logger.Info("An info message")
	// info.log contains:
	// [INFO] 2020/09/06 12:01 An info message
}
