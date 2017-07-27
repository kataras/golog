package benchmarks

import (
	"testing"

	"github.com/kataras/golog"
)

var nopOutput = golog.NopOutput

func BenchmarkGologPrint(b *testing.B) {
	// logger defaults
	golog.SetOutput(nopOutput)
	golog.SetLevel("debug")

	b.ReportAllocs()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		printGolog(i)
	}
}

func printGolog(i int) {
	golog.Errorf("[%d] This is an error message", i)
	golog.Warnf("[%d] This is a warning message", i)
	golog.Infof("[%d] This is an error message", i)
	golog.Debugf("[%d] This is a debug message", i)
}
