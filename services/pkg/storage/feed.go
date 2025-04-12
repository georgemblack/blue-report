package storage

import (
	"context"
	"encoding/json"
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
	Content   FeedEntryContent // Stored as JSON blob
	Timestamp time.Time
}

type FeedEntryContent struct {
	Title            string          `json:"title"`
	URL              string          `json:"url"`
	RecommendedPosts []FeedEntryPost `json:"recommended_posts"`
}

type FeedEntryPost struct {
	Rank     int    `json:"rank"`
	AtURI    string `json:"at_uri"`
	Username string `json:"username"`
	Handle   string `json:"handle"`
	Text     string `json:"text"`
}

// AddFeedEntry creates a new entry in the DynamoDB 'feed' table.
// If the entry already exists, we do not want to modify it.
func (a AWS) AddFeedEntry(entry FeedEntry) error {
	hashedURL := util.Hash(entry.Content.URL)
	content, err := json.Marshal(entry.Content)
	if err != nil {
		return util.WrapErr("failed to marshal feed entry content", err)
	}

	_, err = a.dynamoDB.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(a.cfg.FeedTableName),
		Item: map[string]dynamoDBTypes.AttributeValue{
			"urlHash":   &dynamoDBTypes.AttributeValueMemberS{Value: hashedURL},
			"timestamp": &dynamoDBTypes.AttributeValueMemberS{Value: entry.Timestamp.Format(time.RFC3339)},
			"published": &dynamoDBTypes.AttributeValueMemberBOOL{Value: false},
			"content":   &dynamoDBTypes.AttributeValueMemberS{Value: string(content)},
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

		// Convert timestamp to time.Time
		ts, err := time.Parse(time.RFC3339, item["timestamp"].(*dynamoDBTypes.AttributeValueMemberS).Value)
		if err != nil {
			return nil, util.WrapErr("failed to parse timestamp", err)
		}

		// Unmarshal content JSON
		var content FeedEntryContent
		if err := json.Unmarshal([]byte(item["content"].(*dynamoDBTypes.AttributeValueMemberS).Value), &content); err != nil {
			return nil, util.WrapErr("failed to unmarshal feed entry content", err)
		}

		entries[i] = FeedEntry{
			Timestamp: ts,
			Content:   content,
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
		Bucket:       aws.String(a.cfg.PublicBucketName),
		Key:          aws.String("feeds/top-day.xml"),
		Body:         strings.NewReader(atom),
		ContentType:  aws.String("application/atom+xml"),
		CacheControl: aws.String("public; max-age=600"), // 10 minutes
	})
	if err != nil {
		return util.WrapErr("failed to put atom feed", err)
	}

	_, err = a.r2.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:       aws.String(a.cfg.PublicBucketName),
		Key:          aws.String("feeds/top-day.json"),
		Body:         strings.NewReader(json),
		ContentType:  aws.String("application/feed+json"),
		CacheControl: aws.String("public; max-age=600"), // 10 minutes
	})
	if err != nil {
		return util.WrapErr("failed to put json feed", err)
	}

	return nil
}

// CleanFeed removes entries from the feed table that are older than 90 days.
func (a AWS) CleanFeed() error {
	entries, err := a.GetFeedEntries()
	if err != nil {
		return util.WrapErr("failed to get feed entries", err)
	}

	for _, entry := range entries {
		if time.Since(entry.Timestamp) > 90*24*time.Hour {
			hashedURL := util.Hash(entry.Content.URL)
			_, err := a.dynamoDB.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
				TableName: aws.String(a.cfg.FeedTableName),
				Key: map[string]dynamoDBTypes.AttributeValue{
					"urlHash": &dynamoDBTypes.AttributeValueMemberS{Value: hashedURL},
				},
			})
			if err != nil {
				return util.WrapErr("failed to delete feed entry", err)
			}
		}
	}

	return nil
}
