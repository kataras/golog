package main

import (
	"context"
	"fmt"

	"github.com/kataras/golog"

	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
	"golang.ngrok.com/ngrok/log"
)

// $ go get golang.ngrok.com/ngrok@latest
// $ go run main.go

type ngrokLoggerAdapter struct {
	logger *golog.Logger
}

// Log(context context.Context, level LogLevel, msg string, data map[string]any)

// Log implements the ngrok.Logger interface.
func (l *ngrokLoggerAdapter) Log(context context.Context, level log.LogLevel, msg string, data map[string]any) {
	l.logger.Info(msg, golog.Fields(data))
}

func main() {
	logger := golog.New()

	tun, err := ngrok.Listen(context.Background(),
		config.HTTPEndpoint(
			config.WithDomain("domain.com"),
			// 	config.WithMutualTLSCA(ca),
			config.WithForwardsTo("http://localhost:80"),
			config.WithCircuitBreaker(0.8),
			// config.WithCompression(),
			// config.WithProxyProto(config.ProxyProtoV2),
			config.WithScheme(config.SchemeHTTPS),
		),
		ngrok.WithLogger(&ngrokLoggerAdapter{logger: logger /* OR golog.Default */}),
		ngrok.WithAuthtoken("NGROK_AUTH_TOKEN"),
		// ngrok.WithServer("tunnel.ngrok.com:443"),
		ngrok.WithRegion("us"),
	)
	if err != nil {
		logger.Fatal(err)
	}
	defer tun.Close()

	fmt.Println(tun.URL())
}
