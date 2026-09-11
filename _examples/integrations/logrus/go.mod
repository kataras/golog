module integrations/logrus

go 1.27

replace github.com/kataras/golog => ../../../

require (
	github.com/kataras/golog v0.1.13
	github.com/sirupsen/logrus v1.10.2
)

require golang.org/x/sys v0.47.0 // indirect
