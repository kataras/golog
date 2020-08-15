package main

import (
	"github.com/kataras/golog"
)

func main() {

	golog.Child("Router").Infof("Route %s regirested", "/mypath")
	// registerRoute("/mypath")
	golog.Child("Router").Warnf("Route %s already exists, skipping second registration", "/mypath")

	golog.Error("Something went wrong!")

	var (
		srvLogger  = golog.Child("Server")
		app1Logger = srvLogger.Child("App1")
		// Or use a pointer as child's key and append the prefix manually:
		app2       = newApp("App2")
		app2Logger = srvLogger.Child(app2).
				SetChildPrefix(app2.Name).
				SetLevel("debug")
	)

	srvLogger.Infof("Hello Server")
	app1Logger.Infof("Hello App1")
	app2Logger.Debugf("Hello App2")
}

type app struct {
	Name string
}

func newApp(name string) *app {
	return &app{Name: name}
}
