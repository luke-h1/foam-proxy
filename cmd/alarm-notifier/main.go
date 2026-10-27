package main

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/foam/proxy/internal/alarmnotifier"
	"github.com/foam/proxy/internal/config"
)

func main() {
	config.InitLogger()

	cfg, err := alarmnotifier.LoadConfig()
	if err != nil {
		slog.Error("load alarm notifier config", "err", err)
		os.Exit(1)
	}

	notifier := alarmnotifier.NewNotifier(cfg)
	lambda.Start(notifier.HandleSNS)
}
