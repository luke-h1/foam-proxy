package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/foam/proxy/internal/authorizer"
)

func main() {
	authorizer.InitSentry()
	handler := authorizer.NewHandler()
	lambda.Start(handler.HandleRequest)
}
