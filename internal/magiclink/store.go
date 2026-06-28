// Package magiclink stores the App Review magic-link token blob in SSM Parameter
// Store (SecureString). It is the canonical store both the proxy (read) and the
// magic-keepalive Lambda (read + write) share, replacing the GitHub-secret
// write-back the old refresh workflow relied on. SSM survives terraform applies
// (the parameter is seeded with ignore_changes on its value), so a deploy never
// reverts a rotated token.
package magiclink

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// Store reads and writes the raw blob JSON for a single SSM parameter.
type Store struct {
	client *ssm.Client
	param  string
}

// NewStore resolves AWS config from the ambient environment (region + role on
// Lambda) and binds the store to the given parameter name.
func NewStore(ctx context.Context, param string) (*Store, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &Store{client: ssm.NewFromConfig(cfg), param: param}, nil
}

// Get returns the decrypted blob JSON.
func (s *Store) Get(ctx context.Context) (string, error) {
	out, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(s.param),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Parameter.Value), nil
}

// Put overwrites the parameter with a new blob, keeping it a SecureString.
func (s *Store) Put(ctx context.Context, value string) error {
	_, err := s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(s.param),
		Value:     aws.String(value),
		Type:      types.ParameterTypeSecureString,
		Overwrite: aws.Bool(true),
	})
	return err
}
