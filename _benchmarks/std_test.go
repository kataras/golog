// +build ignore

package benchmarks

import (
	"log"
	"testing"
	"time"

	"github.com/kataras/pio"
)

func BenchmarkStdPrint(b *testing.B) {
	// logrus defaults
	log.SetOutput(nopOutput)

	b.ReportAllocs()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		// time.Now is being called on both golog and logrus, so it's fair to put it here
		// this is a small adition, it doesn't makes the comparison fair but it's a small step.
		_ = time.Now().Format("")

		printStd(i)
	}
}

func printStd(i int) {
	log.Printf("[%d] [%s] This is an error message\n", i, pio.Red("[ERRO]"))
	log.Printf("[%d] [%s] This is a warning message\n", i, pio.Purple("[WARN]"))
	log.Printf("[%d] [%s] This is an info message\n", i, pio.LightGreen("[INFO]"))
	log.Printf("[%d] [%s] This is a debug message\n", i, pio.Yellow("[DBUG]"))
}
