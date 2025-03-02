package storage

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoDBTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

	_, err := a.dynamoDB.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String(a.feedTableName),
		Key: map[string]dynamoDBTypes.AttributeValue{
			"urlHash": &dynamoDBTypes.AttributeValueMemberS{Value: hashedURL},
		},
	})
	if err == nil {
		return nil // Entry already exists
	}

	_, err = a.dynamoDB.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(a.feedTableName),
		Item: map[string]dynamoDBTypes.AttributeValue{
			"url":       &dynamoDBTypes.AttributeValueMemberS{Value: entry.URL},
			"urlHash":   &dynamoDBTypes.AttributeValueMemberS{Value: hashedURL},
			"postId":    &dynamoDBTypes.AttributeValueMemberS{Value: entry.PostID},
			"timestamp": &dynamoDBTypes.AttributeValueMemberS{Value: ts},
			"published": &dynamoDBTypes.AttributeValueMemberBOOL{Value: false},
		},
	})
	if err != nil {
		return util.WrapErr("failed to put url metadata", err)
	}

	return nil
}

func (a AWS) MarkFeedEntryPublished(urlHash string) error {
	_, err := a.dynamoDB.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName: aws.String(a.feedTableName),
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
