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
	// disable time formatting because logrus and std doesn't print the time.
	// note that the time is being set-ed to time.Now() inside the golog's Log structure, same for logrus,
	// Therefore we set the time format to empty on golog test in order
	// to acomblish a fair comparison between golog and logrus.
	golog.SetTimeFormat("")

	b.ReportAllocs()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		printGolog(i)
	}
}

func printGolog(i int) {
	golog.Errorf("[%d] This is an error message", i)
	golog.Warnf("[%d] This is a warning message", i)
	golog.Infof("[%d] This is an info message", i)
	golog.Debugf("[%d] This is a debug message", i)
}
