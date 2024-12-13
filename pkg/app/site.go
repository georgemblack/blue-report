package app

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func siteBucketName() string {
	return getEnvStr("S3_BUCKET_NAME", "bogus")
}

func assetsBucketName() string {
	return getEnvStr("S3_ASSETS_BUCKET_NAME", "bogus")
}

func publish(site []byte) error {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		return wrapErr("failed to load aws config", err)
	}
	client := s3.NewFromConfig(cfg)

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(siteBucketName()),
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

func writeCache(dump Dump) error {
	// Convert dump to JSON
	data, err := json.Marshal(dump)
	if err != nil {
		return wrapErr("failed to marshal dump", err)
	}

	// Build S3 client
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		return wrapErr("failed to load aws config", err)
	}
	client := s3.NewFromConfig(cfg)

	// Write cache to S3
	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(assetsBucketName()),
		Key:                  aws.String("cache.json"),
		Body:                 bytes.NewReader(data),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("application/json"),
	})
	if err != nil {
		return wrapErr("failed to put object", err)
	}

	return nil
}

func readCache() (Dump, error) {
	// Build S3 client
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		return Dump{}, wrapErr("failed to load aws config", err)
	}
	client := s3.NewFromConfig(cfg)

	// Read cache from S3
	resp, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(assetsBucketName()),
		Key:    aws.String("cache.json"),
	})
	if err != nil {
		return Dump{}, wrapErr("failed to get object", err)
	}

	// Decode JSON
	var dump Dump
	err = json.NewDecoder(resp.Body).Decode(&dump)
	if err != nil {
		return Dump{}, wrapErr("failed to decode json", err)
	}

	return dump, nil
}
