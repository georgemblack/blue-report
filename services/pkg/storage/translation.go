package storage

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoDBTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/georgemblack/blue-report/pkg/util"
)

type URLTranslation struct {
	Source      string
	Destination string
}

func (a AWS) SaveURLTranslation(translation URLTranslation) error {
	now := time.Now().UTC()
	ts := now.Format(time.RFC3339Nano)
	month := now.Format("2006-01")

	_, err := a.dynamoDB.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(a.cfg.URLTranslationsTableName),
		Item: map[string]dynamoDBTypes.AttributeValue{
			"updatedAt":      &dynamoDBTypes.AttributeValueMemberS{Value: ts},
			"updatedAtMonth": &dynamoDBTypes.AttributeValueMemberS{Value: month},
			"sourceUrl":      &dynamoDBTypes.AttributeValueMemberS{Value: translation.Source},
			"destinationUrl": &dynamoDBTypes.AttributeValueMemberS{Value: translation.Destination},
		},
	})
	if err != nil {
		return util.WrapErr("failed to put url metadata", err)
	}
	return nil
}

func (a AWS) GetURLTranslations() (map[string]string, error) {
	now := time.Now().UTC()
	formatted := now.Format(time.RFC3339Nano)
	thisMonth := now.Format("2006-01")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")

	translations := make(map[string]string)

	// Perform two queries to fetch the translations for the current & previous month.
	for _, month := range []string{thisMonth, lastMonth} {
		resp, err := a.dynamoDB.Query(context.Background(), &dynamodb.QueryInput{
			TableName:              aws.String(a.cfg.URLTranslationsTableName),
			KeyConditionExpression: aws.String("updatedAtMonth = :month and updatedAt < :now"),
			ExpressionAttributeValues: map[string]dynamoDBTypes.AttributeValue{
				":month": &dynamoDBTypes.AttributeValueMemberS{Value: month},
				":now":   &dynamoDBTypes.AttributeValueMemberS{Value: formatted},
			},
		})
		if err != nil {
			return nil, util.WrapErr("failed to query url translations", err)
		}

		for _, item := range resp.Items {
			source := item["sourceUrl"].(*dynamoDBTypes.AttributeValueMemberS).Value
			destination := item["destinationUrl"].(*dynamoDBTypes.AttributeValueMemberS).Value
			translations[source] = destination
		}
	}

	return translations, nil
}
