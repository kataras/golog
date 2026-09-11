module github.com/kataras/golog/_examples

go 1.27

replace github.com/kataras/golog => ../

require (
	github.com/kataras/golog v0.1.15
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
)

require (
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/lestrrat-go/strftime v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
