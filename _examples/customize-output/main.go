package main

import "github.com/kataras/golog"

func main() {
	golog.SetLevel("debug")

	golog.SetFormat("json", "    ") // < --
	// To register a custom formatter:
	// golog.RegisterFormatter(golog.Formatter...)
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
	}) // If more than one golog.Fields passed, then they are merged into a single map.

	/* Example Output:
	{
	    "timestamp": 1591423477,
	    "level": "info",
	    "message": "An info message",
	    "fields": {
	        "home": "https://iris-go.com"
	    }
		"stacktrace": [...]
	}
	*/
	golog.Infof("An info message", golog.Fields{"home": "https://iris-go.com"})

	golog.Warnf("Hey, warning here")
	golog.Errorf("Something went wrong!")

	// You can also pass custom structs, like normally you would do.
	type myCustomData struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	golog.Fatalf("A fatal error for %s screen!", "home", golog.Fields{"data": myCustomData{
		Username: "kataras",
		Email:    "kataras2006@hotmail.com",
	}})
}

/* Manually, use it for any custom format:
golog.Handle(jsonOutput)

func jsonOutput(l *golog.Log) bool {
	enc := json.NewEncoder(l.Logger.Printer)
	enc.SetIndent("", "    ")
	err := enc.Encode(l)
	return err == nil
}
*/
