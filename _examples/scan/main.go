package main

import (
	"os"
	"time"

	"github.com/kataras/golog"
)

func main() {
	f := newLogFile()
	defer f.Close()

	golog.SetOutput(f)
	_ = golog.Scan(os.Stdin)
	// type and enter one or more sentences to your console,
	// wait 10 seconds and open the .txt file.
	<-time.After(10 * time.Second)
}

// get a filename based on the date, file logs works that way the most times
// but these are just a sugar.
func todayFilename() string {
	today := time.Now().Format("Jan 02 2006")
	return today + ".txt"
}

func newLogFile() *os.File {
	filename := todayFilename()
	// open an output file, this will append to the today's file if server restarted.
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	return f
}
