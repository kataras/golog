package benchmarks

import (
	"testing"

	"github.com/kataras/golog"
)

var nopOutput = golog.NopOutput

func BenchmarkPrintLogger(b *testing.B) {
	// logger defaults
	golog.SetOutput(nopOutput)
	golog.SetLevel("debug")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printLogger(i)
	}
}

func printLogger(i int) {
	golog.Errorf("[%d] This is an error message", i)
	golog.Warnf("[%d] This is a warning message", i)
	golog.Infof("[%d] This is an error message", i)
	golog.Debugf("[%d] This is a debug message", i)
}

// go test -run=XXX -bench=PrintLogger -benchtime=20s
//
// with disabled output(Default.Printer.SetOutput(NopOutput)):
/*
BenchmarkPrint-8         3000000             10943 ns/op
PASS
ok      github.com/kataras/pio/_examples/custom-logger/logger        43.974s
*/
//
// with output = os.Stderr:
/*
BenchmarkPrint-8         200000            187067 ns/op
PASS
ok      github.com/kataras/pio/_examples/custom-logger/logger        52.588s
*/
