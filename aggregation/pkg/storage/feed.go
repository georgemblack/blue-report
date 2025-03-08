package storage

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoDBTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/georgemblack/blue-report/pkg/util"
)

type FeedEntry struct {
	URL       string
	PostID    string // The top post associated with the given URL
	Timestamp time.Time
}

// AddFeedEntry creates a new entry in the DynamoDB 'feed' table.
// If the entry already exists, we do not want to modify it.
func (a AWS) AddFeedEntry(entry FeedEntry) error {
	hashedURL := util.Hash(entry.URL)

	_, err := a.dynamoDB.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(a.cfg.FeedTableName),
		Item: map[string]dynamoDBTypes.AttributeValue{
			"url":       &dynamoDBTypes.AttributeValueMemberS{Value: entry.URL},
			"urlHash":   &dynamoDBTypes.AttributeValueMemberS{Value: hashedURL},
			"postId":    &dynamoDBTypes.AttributeValueMemberS{Value: entry.PostID},
			"timestamp": &dynamoDBTypes.AttributeValueMemberS{Value: entry.Timestamp.Format(time.RFC3339)},
			"published": &dynamoDBTypes.AttributeValueMemberBOOL{Value: false},
		},
		ConditionExpression: aws.String("attribute_not_exists(urlHash)"), // Do not overwrite existing entries
	})

	if err != nil {
		var awsErr smithy.APIError
		if errors.As(err, &awsErr) && awsErr.ErrorCode() == "ConditionalCheckFailedException" {
			return nil // Not an error, just means the entry already exists
		}
		return util.WrapErr("failed to put url metadata", err)
	}

	return nil
}

func (a AWS) GetFeedEntries() ([]FeedEntry, error) {
	res, err := a.dynamoDB.Scan(context.Background(), &dynamodb.ScanInput{
		TableName: aws.String(a.cfg.FeedTableName),
	})
	if err != nil {
		return nil, util.WrapErr("failed to scan feed table", err)
	}

	entries := make([]FeedEntry, len(res.Items))
	for i, item := range res.Items {
		ts, err := time.Parse(time.RFC3339, item["timestamp"].(*dynamoDBTypes.AttributeValueMemberS).Value)
		if err != nil {
			return nil, util.WrapErr("failed to parse timestamp", err)
		}
		entries[i] = FeedEntry{
			URL:       item["url"].(*dynamoDBTypes.AttributeValueMemberS).Value,
			PostID:    item["postId"].(*dynamoDBTypes.AttributeValueMemberS).Value,
			Timestamp: ts,
		}
	}

	return entries, nil
}

// RecentFeedEntry determies if a feed entry has been added within the last X hours.
// This is used to limit the number of items published if there's a low-engagement day and many links are trending.
func (a AWS) RecentFeedEntry() bool {
	entries, err := a.GetFeedEntries()
	if err != nil {
		slog.Error("failed to get feed entries", "error", err)
		return false
	}

	for _, entry := range entries {
		if time.Since(entry.Timestamp) < 12*time.Hour {
			return true
		}
	}

	return false
}

func (a AWS) PublishFeeds(atom, json string) error {
	_, err := a.r2.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(a.cfg.PublicBucketName),
		Key:         aws.String("feeds/top-day.xml"),
		Body:        strings.NewReader(atom),
		ContentType: aws.String("application/atom+xml"),
	})
	if err != nil {
		return util.WrapErr("failed to put atom feed", err)
	}

	_, err = a.r2.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(a.cfg.PublicBucketName),
		Key:         aws.String("feeds/top-day.json"),
		Body:        strings.NewReader(json),
		ContentType: aws.String("application/feed+json"),
	})
	if err != nil {
		return util.WrapErr("failed to put json feed", err)
	}

	return nil
}
