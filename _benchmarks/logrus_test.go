package benchmarks

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func BenchmarkLogrusPrint(b *testing.B) {
	// logrus defaults
	logrus.SetOutput(nopOutput)
	logrus.SetLevel(logrus.DebugLevel)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printLogrus(i)
	}
}

func printLogrus(i int) {
	logrus.Errorf("[%d] This is an error message", i)
	logrus.Warnf("[%d] This is a warning message", i)
	logrus.Infof("[%d] This is an error message", i)
	logrus.Debugf("[%d] This is a debug message", i)
}

// go test -run=XXX -bench=PrintLogrus -benchtime=20s
// logrus with disabled output and no struct pass, but same methods (Errorf, Warnf,Infof,Debugf)
/*
BenchmarkLogrusPrint-8           3000000              9261 ns/op
PASS
ok      github.com/kataras/pio/_examples/custom-logger/logger        37.239s
*/
