package magiclink

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type Store struct {
	client *ssm.Client
	param  string
}

func NewStore(ctx context.Context, param string) (*Store, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &Store{
		client: ssm.NewFromConfig(cfg),
		param:  param,
	}, nil
}

// return the decrypted blob
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

// put - overwrite param with the new blob
func (s *Store) Put(ctx context.Context, value string) error {
	_, err := s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(s.param),
		Value:     aws.String(value),
		Type:      types.ParameterTypeSecureString,
		Overwrite: aws.Bool(true),
	})
	return err
}
