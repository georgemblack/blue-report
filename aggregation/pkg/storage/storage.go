package storage

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/georgemblack/blue-report/pkg/config"
	"github.com/georgemblack/blue-report/pkg/util"
)

type AWS struct {
	s3       *s3.Client
	r2       *s3.Client
	dynamoDB *dynamodb.Client
	cfg      config.Config
}

func New(cfg config.Config) (AWS, error) {
	// Configuration for AWS S3 and DynamoDB client
	awsCfg, err := awsConfig.LoadDefaultConfig(context.Background(), awsConfig.WithRegion("us-west-2"))
	if err != nil {
		return AWS{}, util.WrapErr("failed to load aws config", err)
	}

	// Configuration and client for Cloudflare R2 storage.
	// R2 is mostly API-compatible with S3, thus an S3 client is used.
	r2Creds := credentials.NewStaticCredentialsProvider(cfg.CloudflareR2AccessKeyID, cfg.CloudflareR2SecretAccessKey, "")
	r2Cfg, err := awsConfig.LoadDefaultConfig(context.Background(), awsConfig.WithRegion("auto"), awsConfig.WithCredentialsProvider(r2Creds))
	if err != nil {
		return AWS{}, util.WrapErr("failed to load aws config", err)
	}
	r2Client := s3.NewFromConfig(r2Cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.CloudflareAccountID))
	})

	return AWS{
		s3:       s3.NewFromConfig(awsCfg),
		r2:       r2Client,
		dynamoDB: dynamodb.NewFromConfig(awsCfg),
		cfg:      cfg,
	}, nil
}
