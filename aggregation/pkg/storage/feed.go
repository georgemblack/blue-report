package storage

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoDBTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/smithy-go"
	"github.com/georgemblack/blue-report/pkg/util"
)

type FeedEntry struct {
	URL    string
	PostID string // The top post associated with the given URL
}

// AddFeedEntry creates a new entry in the DynamoDB 'feed' table.
// If the entry already exists, we do not want to modify it.
func (a AWS) AddFeedEntry(entry FeedEntry) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	hashedURL := util.Hash(entry.URL)

	_, err := a.dynamoDB.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(a.cfg.FeedTableName),
		Item: map[string]dynamoDBTypes.AttributeValue{
			"url":       &dynamoDBTypes.AttributeValueMemberS{Value: entry.URL},
			"urlHash":   &dynamoDBTypes.AttributeValueMemberS{Value: hashedURL},
			"postId":    &dynamoDBTypes.AttributeValueMemberS{Value: entry.PostID},
			"timestamp": &dynamoDBTypes.AttributeValueMemberS{Value: ts},
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

func (a AWS) MarkFeedEntryPublished(urlHash string) error {
	_, err := a.dynamoDB.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName: aws.String(a.cfg.FeedTableName),
		Key: map[string]dynamoDBTypes.AttributeValue{
			"urlHash": &dynamoDBTypes.AttributeValueMemberS{Value: urlHash},
		},
		ExpressionAttributeValues: map[string]dynamoDBTypes.AttributeValue{
			":published": &dynamoDBTypes.AttributeValueMemberBOOL{Value: true},
		},
		UpdateExpression: aws.String("SET published = :published"),
	})
	if err != nil {
		return util.WrapErr("failed to update feed item", err)
	}
	return nil
}
