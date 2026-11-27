package hydrate

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Titles remembers the title found for each link, so each page is only
// looked up once.
type Titles interface {
	// Get returns the saved title, or "" if there isn't one.
	Get(ctx context.Context, url string) (string, error)
	Put(ctx context.Context, url, title string) error
}

// DynamoTitles keeps titles in v1's DynamoDB table (blue-report-url-metadata),
// so v1 and v2 share them. Items look like
// {urlHash: <urlKey>, url: <url>, title: <title>}.
type DynamoTitles struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoTitles(client *dynamodb.Client, table string) *DynamoTitles {
	return &DynamoTitles{client: client, table: table}
}

func (d *DynamoTitles) Get(ctx context.Context, url string) (string, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.table),
		Key:       map[string]types.AttributeValue{"urlHash": &types.AttributeValueMemberS{Value: urlKey(url)}},
	})
	if err != nil {
		return "", fmt.Errorf("getting title for %s: %w", url, err)
	}
	if title, ok := out.Item["title"].(*types.AttributeValueMemberS); ok {
		return title.Value, nil
	}
	return "", nil
}

func (d *DynamoTitles) Put(ctx context.Context, url, title string) error {
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.table),
		Item: map[string]types.AttributeValue{
			"urlHash": &types.AttributeValueMemberS{Value: urlKey(url)},
			"url":     &types.AttributeValueMemberS{Value: url},
			"title":   &types.AttributeValueMemberS{Value: title},
		},
	})
	if err != nil {
		return fmt.Errorf("saving title for %s: %w", url, err)
	}
	return nil
}

// MemoryTitles keeps titles in memory, for local runs and tests.
type MemoryTitles struct {
	mu     sync.Mutex
	titles map[string]string
}

func NewMemoryTitles() *MemoryTitles {
	return &MemoryTitles{titles: map[string]string{}}
}

func (m *MemoryTitles) Get(_ context.Context, url string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.titles[url], nil
}

func (m *MemoryTitles) Put(_ context.Context, url, title string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.titles[url] = title
	return nil
}
