package main

import (
	"time"

	"github.com/kataras/golog"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

/*
	go get github.com/lestrrat-go/file-rotatelogs
*/

func main() {
	// Read more at: https://github.com/lestrrat-go/file-rotatelogs#synopsis
	pathToAccessLog := "./access_log.%Y%m%d%H%M"
	w, err := rotatelogs.New(
		pathToAccessLog,
		rotatelogs.WithMaxAge(24*time.Hour),
		rotatelogs.WithRotationTime(time.Hour))
	if err != nil {
		golog.Fatal(err)
	}

	golog.SetOutput(w)

	golog.Println("A Log entry")
}
