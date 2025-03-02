package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoDBTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/georgemblack/blue-report/pkg/config"
	"github.com/georgemblack/blue-report/pkg/util"
)

type AWS struct {
	s3                       *s3.Client
	dynamoDB                 *dynamodb.Client
	publicBucketName         string
	readEventsBucketName     string
	writeEventsBucketName    string
	urlMetadataTableName     string
	urlTranslationsTableName string
	feedTableName            string
}

func New(cfg config.Config) (AWS, error) {
	config, err := awsConfig.LoadDefaultConfig(context.Background(), awsConfig.WithRegion("us-west-2"))
	if err != nil {
		return AWS{}, util.WrapErr("failed to load aws config", err)
	}

	return AWS{
		s3:                       s3.NewFromConfig(config),
		dynamoDB:                 dynamodb.NewFromConfig(config),
		publicBucketName:         cfg.PublicBucketName,
		readEventsBucketName:     cfg.ReadEventsBucketName,
		writeEventsBucketName:    cfg.WriteEventsBucketName,
		urlMetadataTableName:     cfg.URLMetadataTableName,
		urlTranslationsTableName: cfg.URLTranslationsTableName,
		feedTableName:            cfg.FeedTableName,
	}, nil
}

// PublishLinkSnapshot publishes the snapshot of the site's data to S3.
// Store a 'latest' version, as well as a timestamped version.
func (a AWS) PublishLinkSnapshot(snapshot []byte) error {
	_, err := a.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(a.publicBucketName),
		Key:                  aws.String("snapshot.json"),
		Body:                 bytes.NewReader(snapshot),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("application/json"),
		CacheControl:         aws.String("max-age=600"), // 10 minutes
	})
	if err != nil {
		return util.WrapErr("failed to put object", err)
	}

	// Add or update today's page in the archive.
	// Use Eastern time, as the site is primarily for a US audience.
	now := util.ToEastern(time.Now())
	path := fmt.Sprintf("snapshots/%d/%d/%d/snapshot.json", now.Year(), now.Month(), now.Day())
	_, err = a.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(a.publicBucketName),
		Key:                  aws.String(path),
		Body:                 bytes.NewReader(snapshot),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("application/json"),
		CacheControl:         aws.String("max-age=3600"), // 1 hour
	})
	if err != nil {
		return util.WrapErr("failed to put object", err)
	}

	return nil
}

// PublishSiteSnapshot publishes the snapshot of the site's data to S3.
func (a AWS) PublishSiteSnapshot(snapshot []byte) error {
	_, err := a.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(a.publicBucketName),
		Key:                  aws.String("sites.json"),
		Body:                 bytes.NewReader(snapshot),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("application/json"),
		CacheControl:         aws.String("max-age=600"), // 10 minutes
	})
	if err != nil {
		return util.WrapErr("failed to put object", err)
	}

	return nil
}

func (a AWS) ReadEvents(key string, eventBufferSize int) ([]EventRecord, error) {
	resp, err := a.s3.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(a.readEventsBucketName),
		Key:    aws.String(fmt.Sprintf("events/%s.json", key)),
	})
	if err != nil {
		return nil, util.WrapErr("failed to get object", err)
	}
	defer resp.Body.Close()

	// Decode JSON lines
	dec := json.NewDecoder(resp.Body)
	events := make([]EventRecord, 0, eventBufferSize)
	for {
		event := EventRecord{}
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

func (a AWS) FlushEvents(start time.Time, events []EventRecord) error {
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
	_, err := a.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(a.writeEventsBucketName),
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
func (a AWS) ListEventChunks(start, end time.Time) ([]string, error) {
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
		paginator := s3.NewListObjectsV2Paginator(a.s3, &s3.ListObjectsV2Input{
			Bucket: aws.String(a.readEventsBucketName),
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

// SaveThumbnail fetches an image at a given URL and stores it in S3.
// The identifier for the image is returned.
func (a AWS) SaveThumbnail(id string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return util.WrapErr("failed to fetch image", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch image, status code: %s", resp.Status)
	}
	defer resp.Body.Close()

	image, err := io.ReadAll(resp.Body)
	if err != nil {
		return util.WrapErr("failed to read image", err)
	}

	key := fmt.Sprintf("thumbnails/%s.jpg", id)
	_, err = a.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(a.publicBucketName),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(image),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("image/jpeg"),
		CacheControl:         aws.String("max-age=86400"), // 1 day
	})
	if err != nil {
		return util.WrapErr("failed to put object", err)
	}

	return nil
}

func (a AWS) ThumbnailExists(id string) (bool, error) {
	_, err := a.s3.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(a.publicBucketName),
		Key:    aws.String(fmt.Sprintf("thumbnails/%s.jpg", id)),
	})
	if err != nil {
		var notFound *s3Types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, util.WrapErr("failed to head object", err)
	}

	return true, nil
}

func (a AWS) GetURLMetadata(url string) (URLMetadata, error) {
	resp, err := a.dynamoDB.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String("blue-report-url-metadata"),
		Key:       map[string]dynamoDBTypes.AttributeValue{"urlHash": &dynamoDBTypes.AttributeValueMemberS{Value: util.Hash(url)}},
	})
	if err != nil {
		return URLMetadata{}, util.WrapErr("failed to get url metadata from dynamodb", err)
	}
	if len(resp.Item) == 0 {
		return URLMetadata{}, nil
	}

	return URLMetadata{
		URL:   url,
		Title: resp.Item["title"].(*dynamoDBTypes.AttributeValueMemberS).Value,
	}, nil
}

func (a AWS) SaveURLMetadata(metadata URLMetadata) error {
	_, err := a.dynamoDB.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(a.urlMetadataTableName),
		Item: map[string]dynamoDBTypes.AttributeValue{
			"urlHash": &dynamoDBTypes.AttributeValueMemberS{Value: util.Hash(metadata.URL)},
			"url":     &dynamoDBTypes.AttributeValueMemberS{Value: metadata.URL},
			"title":   &dynamoDBTypes.AttributeValueMemberS{Value: metadata.Title},
		},
	})
	if err != nil {
		return util.WrapErr("failed to put url metadata", err)
	}

	return nil
}
