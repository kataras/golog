package main

import (
	"github.com/kataras/golog"
)

func main() {

	golog.Child("Router").Infof("Route %s regirested", "/mypath")
	// registerRoute("/mypath")
	golog.Child("Router").Warnf("Route %s already exists, skipping second registration", "/mypath")

	golog.Error("Something went wrong!")
}
