package main

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy"
)

func main() {
	config.InitLogger()
	proxy.InitSentry()

	handler, err := proxy.NewHandler()
	if err != nil {
		slog.Error("proxy init", "err", err)
		os.Exit(1)
	}

	lambda.Start(handler.HandleRequest)
}
