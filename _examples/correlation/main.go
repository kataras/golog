// Correlating log lines with the standard library's uuid package (Go 1.27).
package main

import (
	"uuid"

	"github.com/kataras/golog"
)

func main() {
	golog.SetLevel("debug")
	golog.SetTimeFormat("")

	// A fresh identifier on every entry. NewV7 is the one to reach for in logs:
	// v7 UUIDs carry a timestamp in their leading bits, so the ids sort in the
	// order the lines were written.
	golog.SetIDGenerator(uuid.NewV7)

	golog.Info("each of these lines gets its own id")
	golog.Warn("...and so does this one")

	// A logger whose generator returns a fixed value tags every line it
	// writes with the same id -- which is what a per-request logger wants, so
	// that one request's lines can be picked out of the stream later.
	handleRequest("GET /users")
	handleRequest("POST /users")

	// The "id" field shows up in the JSON formatter too.
	golog.SetFormat("json", "  ")
	golog.Info("as json")
}

func handleRequest(route string) {
	requestID := uuid.NewV7()

	// Clone, not Child: Child caches one logger per key, so two requests on
	// the same route would be setting each other's id. A Clone is this
	// request's alone, and is garbage once the request is done.
	log := golog.Default.Clone().SetIDGenerator(func() uuid.UUID { return requestID })
	log.SetChildPrefix(route)
	log.Info("started")
	log.Info("finished")
}
