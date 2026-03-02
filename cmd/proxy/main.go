package main

import (
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/foam/proxy/internal/proxy"
)

func main() {
	proxy.InitSentry()

	handler, err := proxy.NewHandler()

	if err != nil {
		log.Fatal(err)
	}

	lambda.Start(handler.HandleRequest)
}
