package magiclink

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type fakeSSMClient struct {
	getCalls int
	getErrs  []error
	getOut   *ssm.GetParameterOutput

	putCalls int
	putErrs  []error
}

func (f *fakeSSMClient) GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	err := errAt(f.getErrs, f.getCalls)
	f.getCalls++
	if err != nil {
		return nil, err
	}
	return f.getOut, nil
}

func (f *fakeSSMClient) PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	err := errAt(f.putErrs, f.putCalls)
	f.putCalls++
	if err != nil {
		return nil, err
	}
	return &ssm.PutParameterOutput{}, nil
}

func errAt(errs []error, i int) error {
	if i < len(errs) {
		return errs[i]
	}
	return nil
}

const validBlob = `{"access_token":"ABC","refresh_token":"REF"}`

func TestStoreGetRetriesThenSucceeds(t *testing.T) {
	client := &fakeSSMClient{
		getErrs: []error{errors.New("transient"), errors.New("transient")},
		getOut: &ssm.GetParameterOutput{
			Parameter: &types.Parameter{Value: aws.String(validBlob)},
		},
	}
	store := &Store{client: client, param: "/foo"}

	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if got != validBlob {
		t.Fatalf("Get() = %q, want %q", got, validBlob)
	}
	if client.getCalls != 3 {
		t.Fatalf("getCalls = %d, want 3", client.getCalls)
	}
}

func TestStoreGetRetriesExhausted(t *testing.T) {
	client := &fakeSSMClient{
		getErrs: []error{errors.New("down"), errors.New("down"), errors.New("down")},
	}
	store := &Store{client: client, param: "/foo"}

	_, err := store.Get(context.Background())
	if err == nil {
		t.Fatal("Get() error = nil, want error after retries exhausted")
	}
	if client.getCalls != retryAttempts {
		t.Fatalf("getCalls = %d, want %d", client.getCalls, retryAttempts)
	}
}

func TestStorePutRejectsInvalidBlob(t *testing.T) {
	client := &fakeSSMClient{}
	store := &Store{client: client, param: "/foo"}

	err := store.Put(context.Background(), `not json`)
	if err == nil {
		t.Fatal("Put() error = nil, want error for malformed blob")
	}
	if client.putCalls != 0 {
		t.Fatalf("putCalls = %d, want 0 (SSM should not be called)", client.putCalls)
	}
}

func TestStorePutAcceptsValidBlob(t *testing.T) {
	client := &fakeSSMClient{}
	store := &Store{client: client, param: "/foo"}

	if err := store.Put(context.Background(), validBlob); err != nil {
		t.Fatalf("Put() error = %v, want nil", err)
	}
	if client.putCalls != 1 {
		t.Fatalf("putCalls = %d, want 1", client.putCalls)
	}
}
