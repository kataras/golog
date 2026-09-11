/*
BSD 3-Clause License

Copyright (c) 2017-2026, Gerasimos (Makis) Maropoulos (kataras2006@hotmail.com)
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

 1. Redistributions of source code must retain the above copyright notice, this
    list of conditions and the following disclaimer.

 2. Redistributions in binary form must reproduce the above copyright notice,
    this list of conditions and the following disclaimer in the documentation
    and/or other materials provided with the distribution.

 3. Neither the name of the copyright holder nor the names of its
    contributors may be used to endorse or promote products derived from
    this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

/*
Package golog provides an easy to use foundation for your logging operations.

Source code and other details for the project are available at GitHub:

	https://github.com/kataras/golog

# Current Version

0.2.0

# Installation

The only requirement is the Go Programming Language, version 1.27 or later.

	$ go get github.com/kataras/golog@latest

# Overview

Example code:

	package main

	import (
		"github.com/kataras/golog"
	)

	func main() {
		// Default Output is `os.Stdout`,
		// but you can change it:
		// golog.SetOutput(os.Stderr)

		// Time Format defaults to: "2006/01/02 15:04"
		// you can change it to something else or disable it with:
		golog.SetTimeFormat("")

		// Level defaults to "info",
		// but you can change it:
		golog.SetLevel("debug")

		golog.Println("This is a raw message, no levels, no colors.")
		golog.Info("This is an info message, with colors (if the output is terminal)")
		golog.Warn("This is a warning message")
		golog.Error("This is an error message")
		golog.Debug("This is a debug message")
	}

# New

Golog has a default, package-level initialized instance for you,
however you can choose to create and use a logger instance for a
specific part of your application.

Example Code:

	package main

	import (
		"github.com/kataras/golog"
	)

	func main() {
		log := golog.New()

		// Default Output is `os.Stdout`,
		// but you can change it:
		// log.SetOutput(os.Stderr)

		// Level defaults to "info",
		// but you can change it:
		log.SetLevel("debug")

		log.Println("This is a raw message, no levels, no colors.")
		log.Info("This is an info message, with colors (if the output is terminal)")
		log.Warn("This is a warning message")
		log.Error("This is an error message")
		log.Debug("This is a debug message")
	}

# Format

Golog sets colors to levels when its `Printer.Output` is actual a compatible terminal
which can renders colors, otherwise it will downgrade itself to a white foreground.

Golog has functions to print a formatted log too.

Example Code:

	golog.Infof("[%d] This is an info %s", 1, "formatted log")
	golog.Warnf("[%d] This is an info %s", 1, "formatted log")
	golog.Errorf("[%d] This is an info %s", 1, "formatted log")
	golog.Debugf("[%d] This is an info %s", 1, "formatted log")

# Output

Golog takes a simple `io.Writer` as its underline Printer's Output.

Example Code:

	golog.SetOutput(io.Writer)

# Levels

Golog is a leveled logger, therefore you can set a level and print
whenever the print level is valid with the set-ed one.

Available built'n levels are:

	// DisableLevel will disable printer
	DisableLevel Level = iota
	// FatalLevel will `os.Exit(1)` no matter the level of the logger
	FatalLevel
	// ErrorLevel will print only errors
	ErrorLevel
	// WarnLevel will print errors and warnings
	WarnLevel
	// InfoLevel will print errors, warnings and infos
	InfoLevel
	// DebugLevel will print on any level, errors, warnings, infos and debug messages
	DebugLevel

Below you'll learn a way to add a custom level or modify an existing level.

The default colorful text(or raw text for unsupported outputs) for levels
can be overridden by using the `golog#ErrorText, golog#WarnText, golog#InfoText and golog#DebugText`
functions.

Example Code:

	package main

	import (
		"github.com/kataras/golog"
	)

	func main() {

		// First argument is the raw text for outputs
		// second argument is the color code
		// and the last, variadic argument can be any `golog/printer.RichOption`,
		// e.g. printer.Background, printer.Underline, printer.Bold.

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

Golog gives you the power to add or modify existing levels is via Level Metadata.

Example Code:

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
		// Register our level, just three fields. RegisterLevel is safe to
		// call at any time; writing to golog.Levels directly also works,
		// as long as it happens before logging starts.
		golog.RegisterLevel(SuccessLevel, &golog.LevelMetadata{
			Name:      "success",
			Title:     "[SUCC]",
			ColorCode: 32, // Green
		})

		// create a new golog logger
		myLogger := golog.New()

		// set its level to the higher in order to see it
		// ("success" is the name we gave to our level)
		myLogger.SetLevel("success")

		// and finally print a log message with our custom level
		myLogger.Logf(SuccessLevel, "This is a success log message with green color")
	}

The logger's level can be changed via passing one of the
level constants to the `Level` field or by
passing its string representation to the `SetLevel` function.

Example Code:

	golog.SetLevel("disable")
	golog.SetLevel("fatal")
	golog.SetLevel("error")
	golog.SetLevel("warn")
	golog.SetLevel("info")
	golog.SetLevel("debug")

# Integrations

Transaction with your favorite, but deprecated logger is easy.
Golog offers two basic interfaces, the `ExternalLogger` and the `StdLogger`
that can be directly used as arguments to the `Install` function
in order to adapt an external logger.

Outline:

	// Install receives  an external logger
	// and automatically adapts its print functions.
	//
	// Install adds a golog handler to support third-party integrations,
	// it can be used only once per `golog#Logger` instance.
	//
	// For example, if you want to print using a logrus
	// logger you can do the following:
	//
	//	Install(logrus.StandardLogger())
	//
	// Or the standard log's Logger:
	//
	//	import "log"
	//	myLogger := log.New(os.Stdout, "", 0)
	//	Install(myLogger)
	//
	// Or even the slog/log's Logger:
	//
	//	import "log/slog"
	//	myLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	//	Install(myLogger) OR Install(slog.Default())
	//
	// Look `golog#Logger.Handle` for more.
	Install(logger any)

# Logrus Integration

Example Code:

	package main

	import (
		"github.com/kataras/golog"
		"github.com/sirupsen/logrus"
	)

	func main() {
		// outputOnly()
		full()
	}

	func full() {
		// simulate a logrus preparation:
		logrus.SetLevel(logrus.InfoLevel)
		logrus.SetFormatter(&logrus.JSONFormatter{})

		// pass logrus.StandardLogger() to print logs using using the default,
		// package-level logrus' instance of Logger:
		golog.Install(logrus.StandardLogger())

		golog.Debug(`this debug message will not be shown,
		because the logrus level is InfoLevel`)
		golog.Error("this error message will be visible as json")

		// simulate a change of the logrus formatter
		// as you see we have nothing more to change
		// on the golog, it works out of the box,
		// it will be adapt by this change, automatically.
		logrus.SetFormatter(&logrus.TextFormatter{})

		golog.Error("this error message will be visible as text")
		golog.Info("this info message will be visible as text")
	}

	func outputOnly() {
		golog.SetOutput(logrus.StandardLogger().Out)
		golog.Info(`output only, this will print the same contents
		as golog but using the defined logrus' io.Writer`)

		golog.Error("this error message will be visible as text")
	}

# Standard `log.Logger` Integration

Example Code:

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
		golog.Install(myLogger)

		golog.Debug(`this debug message will not be shown,
		because the golog level is ErrorLevel`)

		golog.Error("this error message will be visible the only visible")

		golog.Warn("this info message will not be visible")
	}

# Log Identifiers

Every Log can carry a unique identifier, taken from the standard library's uuid
package. It is off by default: Log.ID stays the zero UUID and no "id" appears in
any output until a generator is installed.

Example Code:

	package main

	import (
		"uuid"

		"github.com/kataras/golog"
	)

	func main() {
		// NewV7 is the one to reach for in logs: v7 UUIDs carry a timestamp in
		// their leading bits, so the identifiers sort in the order the lines
		// were written.
		golog.SetIDGenerator(uuid.NewV7)

		golog.Info("this line has an id of its own")
	}

Returning a fixed value instead tags every entry of a logger with the same
identifier, which is what a per-request logger wants: one request's lines can
then be picked back out of the stream. Derive that logger with Clone, which
nothing else shares. Child is not the tool here: it caches one logger per key,
so two requests on the same route would be setting each other's id.

	requestID := uuid.NewV7()
	reqLogger := golog.Default.Clone().
		SetIDGenerator(func() uuid.UUID { return requestID })

	reqLogger.Info("started")  // both lines carry
	reqLogger.Info("finished") // the same id

# Typed Fields

Handler and Formatter authors read a log's data out of its Fields map, which is
a map[string]any. Fields.Get and its Log.Field shorthand do the type assertion
for you and report whether it held:

	golog.Handle(func(log *golog.Log) bool {
		username, ok := log.Field[string]("username")
		if ok {
			// [...]
		}

		return false
	})

# That's the basics

But you should have a basic idea of the golog package by now, we just scratched the surface.
If you enjoy what you just saw and want to learn more, please follow the below links:

Examples:

	https://github.com/kataras/golog/tree/master/_examples
*/
package golog

// Version is the version string representation of the "golog" package.
const Version = "0.2.0"
