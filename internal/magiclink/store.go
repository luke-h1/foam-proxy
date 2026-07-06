// Package magiclink stores the App Review magic-link token blob in SSM Parameter
// Store (SecureString). It is the canonical store both the proxy (read) and the
// magic-keepalive Lambda (read + write) share, replacing the GitHub-secret
// write-back the old refresh workflow relied on. SSM survives terraform applies
// (the parameter is seeded with ignore_changes on its value), so a deploy never
// reverts a rotated token.
package magiclink

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/foam/proxy/internal/config"
)

const (
	retryAttempts = 3
	retryBaseWait = 200 * time.Millisecond
)

type ssmClient interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

type Store struct {
	client ssmClient
	param  string
}

func NewStore(ctx context.Context, param string) (*Store, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &Store{client: ssm.NewFromConfig(cfg), param: param}, nil
}

func (s *Store) Get(ctx context.Context) (string, error) {
	out, err := withRetry(ctx, func() (*ssm.GetParameterOutput, error) {
		return s.client.GetParameter(ctx, &ssm.GetParameterInput{
			Name:           aws.String(s.param),
			WithDecryption: aws.Bool(true),
		})
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Parameter.Value), nil
}

func (s *Store) Put(ctx context.Context, value string) error {
	if config.ParseMagicLink(value) == nil {
		return fmt.Errorf("magiclink: refusing to store malformed blob")
	}

	_, err := withRetry(ctx, func() (*ssm.PutParameterOutput, error) {
		return s.client.PutParameter(ctx, &ssm.PutParameterInput{
			Name:      aws.String(s.param),
			Value:     aws.String(value),
			Type:      types.ParameterTypeSecureString,
			Overwrite: aws.Bool(true),
		})
	})
	return err
}

func withRetry[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var (
		result T
		err    error
	)
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}
		if attempt == retryAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(retryBaseWait * time.Duration(attempt)):
		}
	}
	return result, err
}
