package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Storage struct {
	client *s3.Client
}

func NewStorageClient() (Storage, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		return Storage{}, wrapErr("failed to load aws config", err)
	}

	return Storage{client: s3.NewFromConfig(cfg)}, nil
}

func (s Storage) PublishSite(site []byte) error {
	// Update 'index.html', the main site
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(siteBucketName()),
		Key:                  aws.String("index.html"),
		Body:                 bytes.NewReader(site),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("text/html"),
	})
	if err != nil {
		return wrapErr("failed to put object", err)
	}

	// Add or update today's page in the archive
	now := time.Now()
	path := fmt.Sprintf("archive/%d/%d/%d/index.html", now.Year(), now.Month(), now.Day())
	_, err = s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(siteBucketName()),
		Key:                  aws.String(path),
		Body:                 bytes.NewReader(site),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("text/html"),
	})
	if err != nil {
		return wrapErr("failed to put object", err)
	}

	return nil
}

func (s Storage) WriteCache(dump CacheDump) error {
	data, err := json.Marshal(dump)
	if err != nil {
		return wrapErr("failed to marshal dump", err)
	}

	_, err = s.client.PutObject(context.Background(), &s3.PutObjectInput{
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

func (s Storage) ReadCache() (CacheDump, error) {
	resp, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(assetsBucketName()),
		Key:    aws.String("cache.json"),
	})
	if err != nil {
		return CacheDump{}, wrapErr("failed to get object", err)
	}

	var dump CacheDump
	err = json.NewDecoder(resp.Body).Decode(&dump)
	if err != nil {
		return CacheDump{}, wrapErr("failed to decode json", err)
	}

	return dump, nil
}

func siteBucketName() string {
	return getEnvStr("S3_BUCKET_NAME", "bogus")
}

func assetsBucketName() string {
	return getEnvStr("S3_ASSETS_BUCKET_NAME", "bogus")
}
