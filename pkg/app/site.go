package app

import (
	"bytes"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func bucketName() string {
	return getEnvStr("S3_BUCKET_NAME", "bogus")
}

func publish(site []byte) error {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		return wrapErr("failed to load aws config", err)
	}
	client := s3.NewFromConfig(cfg)

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(bucketName()),
		Key:                  aws.String("index.html"),
		Body:                 bytes.NewReader(site),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("text/html"),
	})
	if err != nil {
		return wrapErr("failed to put object", err)
	}

	return nil
}
