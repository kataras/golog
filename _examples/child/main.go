package main

import "github.com/kataras/golog"

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

		// Or use a pointer to a value which implements the fmt.Stringer:
		app3       = newAppWithString("App3")
		app3Logger = srvLogger.Child(app3)
	)

	srvLogger.Infof("Hello Server")
	app1Logger.Infof("Hello App1")
	app2Logger.Debugf("Hello App2")
	app3Logger.Warnf("Hello App3")

	srvLogger.LastChild().Infof("Hello App3 again")
}

type app struct {
	Name string
}

func newApp(name string) *app {
	return &app{Name: name}
}

type appWithString struct {
	name string
}

func newAppWithString(name string) *appWithString {
	return &appWithString{name: name}
}

func (app *appWithString) String() string {
	return app.name
}
