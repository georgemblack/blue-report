package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/georgemblack/blue-report/pkg/app/util"
)

type Storage struct {
	client *s3.Client
}

func NewStorageClient() (Storage, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		return Storage{}, util.WrapErr("failed to load aws config", err)
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
		return util.WrapErr("failed to put object", err)
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
		return util.WrapErr("failed to put object", err)
	}

	return nil
}

func (s Storage) ReadEvents(key string) ([]StorageEventRecord, error) {
	resp, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(assetsBucketName()),
		Key:    aws.String(fmt.Sprintf("events/%s.json", key)),
	})
	if err != nil {
		return nil, util.WrapErr("failed to get object", err)
	}
	defer resp.Body.Close()

	// Decode JSON lines
	dec := json.NewDecoder(resp.Body)
	events := make([]StorageEventRecord, 0)
	for {
		event := StorageEventRecord{}
		if err := dec.Decode(&event); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, util.WrapErr("failed to decode event", err)
		}
		events = append(events, event)
	}

	return events, nil
}

func (s Storage) FlushEvents(start time.Time, events []StorageEventRecord) error {
	// Convert events to JSON lines
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, event := range events {
		err := enc.Encode(event)
		if err != nil {
			return util.WrapErr("failed to encode event", err)
		}
	}

	// Write to S3, with timestamp in key
	key := fmt.Sprintf("events/%s.json", start.UTC().Format("2006-01-02-15-04-05"))
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(assetsBucketName()),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(buf.Bytes()),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("application/json"),
	})
	if err != nil {
		return util.WrapErr("failed to put object", err)
	}

	return nil
}

// ListEventChunks lists all S3 object keys containing events after a certain time.
// Objects are named 'events/<timestamp>.json'.
func (s Storage) ListEventChunks(start, end time.Time) ([]string, error) {
	keys := make([]string, 0)

	// List objects using a list of prefixes, one for each day between 'start' and 'end', inclusive.
	// By using prefixes, we reduce the amount of 'LIST' operations, which can be costly for objects in archival storage classes.
	prefixes := make([]string, 0)
	current := start
	for !current.After(end) {
		prefixes = append(prefixes, fmt.Sprintf("events/%s", current.Format("2006-01-02")))
		current = current.AddDate(0, 0, 1)
	}

	slog.Info(fmt.Sprintf("listing objects with prefixes: %v", prefixes))

	for _, prefix := range prefixes {
		paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
			Bucket: aws.String(assetsBucketName()),
			Prefix: aws.String(prefix),
		})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(context.Background())
			if err != nil {
				return nil, util.WrapErr("failed to list objects", err)
			}

			for _, obj := range page.Contents {
				keys = append(keys, *obj.Key)
			}
		}
	}

	// Filter keys to only include those after the 'start' time, and before the 'end' time.
	filtered := make([]string, 0)
	startStr := start.UTC().Format("2006-01-02-15-04-05")
	endStr := end.UTC().Format("2006-01-02-15-04-05")
	for _, key := range keys {
		// Parse timestamp from key, i.e. 'events/2021-08-01-12-00-00.json' -> '2021-08-01-12-00-00'
		key = strings.TrimPrefix(key, "events/")
		key = strings.TrimSuffix(key, ".json")

		// Compare strings with timestamps
		if key > startStr && key < endStr {
			filtered = append(filtered, key)
		}
	}

	slices.Sort(filtered)

	slog.Info("discovered chunks", "count", len(filtered), "first", keys[0], "last", keys[len(keys)-1])
	return filtered, nil
}

func siteBucketName() string {
	return util.GetEnvStr("S3_BUCKET_NAME", "blue-report-test")
}

func assetsBucketName() string {
	return util.GetEnvStr("S3_ASSETS_BUCKET_NAME", "blue-report-test")
}

type StorageEventRecord struct {
	Type      int       `json:"type"` // 0 = post, 1 = repost, 2 = like
	URL       string    `json:"url"`
	DID       string    `json:"did"`
	Timestamp time.Time `json:"timestamp"`
}

func (s StorageEventRecord) isPost() bool {
	return s.Type == 0
}

func (s StorageEventRecord) isRepost() bool {
	return s.Type == 1
}

func (s StorageEventRecord) isLike() bool {
	return s.Type == 2
}
