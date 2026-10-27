// Package magiclink stores the App Review magic-link token blob in SSM Parameter
// Store (SecureString). It is the store both the proxy (read) and the
// magic-keepalive Lambda (read + write) share, replacing the GitHub-secret
// write-back the old refresh workflow relied on.
package magiclink

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/foam/proxy/internal/config"
)

const (
	retryAttempts = 3
	retryBaseWait = 200 * time.Millisecond
	retryMaxWait  = time.Second
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
		return nil, fmt.Errorf("magiclink: load aws config: %w", err)
	}
	client := ssm.NewFromConfig(cfg, func(o *ssm.Options) {
		o.Retryer = retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 1
		})
	})
	return &Store{client: client, param: param}, nil
}

func (s *Store) Get(ctx context.Context) (string, error) {
	out, err := withRetry(ctx, func() (*ssm.GetParameterOutput, error) {
		return s.client.GetParameter(ctx, &ssm.GetParameterInput{
			Name:           aws.String(s.param),
			WithDecryption: aws.Bool(true),
		})
	})
	if err != nil {
		return "", fmt.Errorf("magiclink: get parameter: %w", err)
	}
	if out == nil || out.Parameter == nil {
		return "", errors.New("magiclink: empty parameter response")
	}
	return aws.ToString(out.Parameter.Value), nil
}

func (s *Store) Put(ctx context.Context, value string) error {
	if config.ParseMagicLink(value) == nil {
		return errors.New("magiclink: refusing to store malformed blob")
	}

	_, err := withRetry(ctx, func() (*ssm.PutParameterOutput, error) {
		return s.client.PutParameter(ctx, &ssm.PutParameterInput{
			Name:      aws.String(s.param),
			Value:     aws.String(value),
			Type:      types.ParameterTypeSecureString,
			Overwrite: aws.Bool(true),
		})
	})
	if err != nil {
		return fmt.Errorf("magiclink: put parameter: %w", err)
	}
	return nil
}

func isRetryable(err error) bool {
	var accessDenied *types.AccessDeniedException
	var notFound *types.ParameterNotFound
	var validation *types.ValidationException
	var invalidKey *types.InvalidKeyId
	var versionLimit *types.ParameterMaxVersionLimitExceeded
	if errors.As(err, &accessDenied) || errors.As(err, &notFound) || errors.As(err, &validation) || errors.As(err, &invalidKey) || errors.As(err, &versionLimit) {
		return false
	}
	return true
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
		if !isRetryable(err) || attempt == retryAttempts {
			break
		}
		if ctx.Err() != nil {
			return result, errors.Join(err, ctx.Err())
		}
		if waitErr := waitBackoff(ctx, backoffDelay(attempt)); waitErr != nil {
			return result, errors.Join(err, waitErr)
		}
	}
	return result, err
}

func backoffCeiling(attempt int) time.Duration {
	ceiling := retryBaseWait << (attempt - 1)
	if ceiling > retryMaxWait {
		return retryMaxWait
	}
	return ceiling
}

func backoffDelay(attempt int) time.Duration {
	ceiling := backoffCeiling(attempt)
	n, err := rand.Int(rand.Reader, big.NewInt(int64(ceiling)))
	if err != nil {
		return ceiling / 2
	}
	return time.Duration(n.Int64())
}

func waitBackoff(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
