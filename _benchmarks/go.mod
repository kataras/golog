module github.com/kataras/golog/_benchmarks

go 1.27

replace github.com/kataras/golog => ../

require (
	github.com/kataras/golog v0.1.15
	github.com/sirupsen/logrus v1.9.3
)

require golang.org/x/sys v0.47.0 // indirect
