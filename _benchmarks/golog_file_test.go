package benchmarks

import (
	"os"
	"testing"

	"github.com/kataras/golog"
)

// BenchmarkGologPrintFile writes to a real file, so the number of Write
// syscalls per line shows up, which a nop writer hides.
func BenchmarkGologPrintFile(b *testing.B) {
	f, err := os.CreateTemp(b.TempDir(), "golog-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()

	logger := golog.New()
	logger.SetOutput(f)
	logger.SetLevel("debug")
	logger.SetTimeFormat("")

	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		logger.Errorf("[%d] This is an error message", i)
		logger.Warnf("[%d] This is a warning message", i)
		logger.Infof("[%d] This is an info message", i)
	}
}
