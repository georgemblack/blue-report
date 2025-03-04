package storage

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoDBTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/georgemblack/blue-report/pkg/util"
)

type URLMetadata struct {
	URL   string
	Title string
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
		TableName: aws.String(a.cfg.URLMetadataTableName),
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
