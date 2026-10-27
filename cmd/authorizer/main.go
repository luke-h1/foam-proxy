package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/foam/proxy/internal/authorizer"
	"github.com/foam/proxy/internal/config"
)

func main() {
	config.InitLogger()
	authorizer.InitSentry()
	handler := authorizer.NewHandler()
	lambda.Start(handler.HandleRequest)
}
