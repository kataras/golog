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

	for i := 0; b.Loop(); i++ {
		printGolog(i)
	}
}

func BenchmarkGologPrintJSON(b *testing.B) {
	logger := golog.New()
	logger.SetOutput(nopOutput)
	logger.SetLevel("debug")
	logger.SetTimeFormat("")
	logger.SetFormat("json", "")

	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		logger.Errorf("[%d] This is an error message", i)
		logger.Warnf("[%d] This is a warning message", i)
		logger.Infof("[%d] This is an info message", i)
	}
}

func printGolog(i int) {
	golog.Errorf("[%d] This is an error message", i)
	golog.Warnf("[%d] This is a warning message", i)
	golog.Infof("[%d] This is an info message", i)
	// Debug on golog prints the whole stacktrace, while logrus does not, don't include that.
}
