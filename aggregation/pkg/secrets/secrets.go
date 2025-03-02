package secrets

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/georgemblack/blue-report/pkg/util"
)

const region = "us-west-2"

type SecretsManager struct {
	client *secretsmanager.Client
}

func New() (SecretsManager, error) {
	config, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return SecretsManager{}, util.WrapErr("failed to load aws config", err)
	}

	return SecretsManager{client: secretsmanager.NewFromConfig(config)}, nil
}

func (s SecretsManager) GetDeployHook() (string, error) {
	return s.getSecret("blue-report/cloudflare-deploy-hook-url")
}

func (s SecretsManager) GetCloudflareAPIToken() (string, error) {
	return s.getSecret("blue-report/cloudflare-api-token")
}

func (s SecretsManager) GetCloudflareAccountID() (string, error) {
	return s.getSecret("blue-report/cloudflare-account-id")
}

func (s SecretsManager) getSecret(secretName string) (string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	result, err := s.client.GetSecretValue(context.Background(), input)
	if err != nil {
		return "", util.WrapErr("failed to get secret value", err)
	}

	var secretString string = *result.SecretString
	return secretString, nil
}
