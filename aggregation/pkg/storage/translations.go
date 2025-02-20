package storage

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoDBTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/georgemblack/blue-report/pkg/util"
)

func (a AWS) SaveURLTranslation(translation URLTranslation) error {
	_, err := a.dynamoDB.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(a.urlTranslationsTableName),
		Item: map[string]dynamoDBTypes.AttributeValue{
			"urlHash":        &dynamoDBTypes.AttributeValueMemberS{Value: util.Hash(translation.Source)},
			"sourceUrl":      &dynamoDBTypes.AttributeValueMemberS{Value: translation.Source},
			"destinationUrl": &dynamoDBTypes.AttributeValueMemberS{Value: translation.Destination},
			"updatedAt":      &dynamoDBTypes.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
		},
	})
	if err != nil {
		return util.WrapErr("failed to put url metadata", err)
	}
	return nil
}
