package benchmarks

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func BenchmarkLogrusPrint(b *testing.B) {
	// logrus defaults
	logrus.SetOutput(nopOutput)
	logrus.SetLevel(logrus.DebugLevel)

	b.ReportAllocs()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		printLogrus(i)
	}
}

func printLogrus(i int) {
	logrus.Errorf("[%d] This is an error message", i)
	logrus.Warnf("[%d] This is a warning message", i)
	logrus.Infof("[%d] This is an info message", i)
	logrus.Debugf("[%d] This is a debug message", i)
}
