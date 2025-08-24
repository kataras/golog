module integrations/logrus

go 1.25

replace github.com/kataras/golog => ../../../

require (
	github.com/kataras/golog v0.1.13
	github.com/sirupsen/logrus v1.9.3
)

require (
	github.com/kataras/pio v0.0.14 // indirect
	golang.org/x/sys v0.31.0 // indirect
)
