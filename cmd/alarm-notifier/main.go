package main

import (
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/foam/proxy/internal/alarmnotifier"
)

func main() {
	cfg, err := alarmnotifier.LoadConfig()

	if err != nil {
		log.Fatal(err)
	}

	notifier := alarmnotifier.NewNotifier(cfg)
	lambda.Start(notifier.HandleSNS)
}
