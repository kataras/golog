package main

import (
	"encoding/json"

	"github.com/kataras/golog"
)

func main() {
	golog.SetLevel("debug")
	golog.Handle(jsonOutput) // <---

	/* Example Output:
	{
	    "timestamp": 1591423477,
	    "level": "debug",
	    "message": "This is a message with data (debug prints the stacktrace too)",
	    "fields": {
	        "username": "kataras"
	    },
	    "stacktrace": [
	        {
	            "function": "main.main",
	            "source": "C:/mygopath/src/github.com/kataras/golog/_examples/customize-output/main.go:29"
	        }
	    ]
	}
	*/
	golog.Debugf("This is a %s with data (debug prints the stacktrace too)", "message", golog.Fields{
		"username": "kataras",
	})

	/* Example Output:
	{
	    "timestamp": 1591423477,
	    "level": "info",
	    "message": "An info message",
	    "fields": {
	        "home": "https://iris-go.com"
	    }
	}
	*/
	golog.Infof("An info message", golog.Fields{"home": "https://iris-go.com"})

	golog.Warnf("Hey, warning here")
	golog.Errorf("Something went wrong!")
}

func jsonOutput(l *golog.Log) bool {
	enc := json.NewEncoder(l.Logger.Printer)
	enc.SetIndent("", "    ")
	err := enc.Encode(l)
	return err == nil
}
