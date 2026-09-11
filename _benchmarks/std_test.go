package benchmarks

import (
	"log"
	"testing"
	"time"

	"github.com/kataras/golog/printer"
)

func BenchmarkStdPrint(b *testing.B) {
	// logrus defaults
	log.SetOutput(nopOutput)

	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		// time.Now is being called on both golog and logrus, so it's fair to put it here
		// this is a small adition, it doesn't makes the comparison fair but it's a small step.
		_ = time.Now().Format("")

		printStd(i)
	}
}

// The level titles the standard library has no concept of, colored the way
// golog colors them, so the three benchmarks are printing the same thing.
//
// This used to call kataras/pio, which golog dropped as a dependency in
// v0.1.14 -- and because _benchmarks was not a module and its directory name
// starts with an underscore, nothing noticed that it had stopped compiling.
var (
	stdErrorTitle = printer.RichText("[ERRO]", printer.Red)
	stdWarnTitle  = printer.RichText("[WARN]", printer.Magenta)
	stdInfoTitle  = printer.RichText("[INFO]", printer.Cyan)
)

func printStd(i int) {
	log.Printf("[%d] [%s] This is an error message\n", i, stdErrorTitle)
	log.Printf("[%d] [%s] This is a warning message\n", i, stdWarnTitle)
	log.Printf("[%d] [%s] This is an info message\n", i, stdInfoTitle)
}
