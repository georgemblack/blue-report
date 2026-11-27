package resolve

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Store keeps what the resolver found for each link. destination is "" for
// a link that doesn't redirect.
type Store interface {
	// Get returns the saved destination, and whether there's a saved answer
	// that hasn't expired by now.
	Get(ctx context.Context, url string, now time.Time) (destination string, found bool, err error)
	Put(ctx context.Context, url, destination string, expires time.Time) error
}

// DynamoStore keeps answers in the blue-report-redirects table. Items look
// like {urlHash, url, destination, expiresAt}, where expiresAt is in seconds
// since 1970. DynamoDB deletes items once they expire, though it can take a
// few days, so Get also checks expiresAt itself.
type DynamoStore struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoStore(client *dynamodb.Client, table string) *DynamoStore {
	return &DynamoStore{client: client, table: table}
}

func (d *DynamoStore) Get(ctx context.Context, url string, now time.Time) (string, bool, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.table),
		Key:       map[string]types.AttributeValue{"urlHash": &types.AttributeValueMemberS{Value: urlKey(url)}},
	})
	if err != nil {
		return "", false, fmt.Errorf("getting redirect for %s: %w", url, err)
	}
	destination, ok := out.Item["destination"].(*types.AttributeValueMemberS)
	expiresAt, ok2 := out.Item["expiresAt"].(*types.AttributeValueMemberN)
	if !ok || !ok2 {
		return "", false, nil
	}
	expires, err := strconv.ParseInt(expiresAt.Value, 10, 64)
	if err != nil || now.Unix() >= expires {
		return "", false, nil
	}
	return destination.Value, true, nil
}

func (d *DynamoStore) Put(ctx context.Context, url, destination string, expires time.Time) error {
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.table),
		Item: map[string]types.AttributeValue{
			"urlHash":     &types.AttributeValueMemberS{Value: urlKey(url)},
			"url":         &types.AttributeValueMemberS{Value: url},
			"destination": &types.AttributeValueMemberS{Value: destination},
			"expiresAt":   &types.AttributeValueMemberN{Value: strconv.FormatInt(expires.Unix(), 10)},
		},
	})
	if err != nil {
		return fmt.Errorf("saving redirect for %s: %w", url, err)
	}
	return nil
}

// urlKey names a link in the table. URLs can be longer than DynamoDB allows
// for a key, so a hash is used instead.
func urlKey(url string) string {
	h := fnv.New64a()
	h.Write([]byte(url))
	return fmt.Sprintf("%x", h.Sum64())
}

// MemoryStore keeps answers in memory, for local runs and tests.
type MemoryStore struct {
	mu    sync.Mutex
	items map[string]memoryItem
}

type memoryItem struct {
	destination string
	expires     time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: map[string]memoryItem{}}
}

func (m *MemoryStore) Get(_ context.Context, url string, now time.Time) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[url]
	if !ok || !now.Before(item.expires) {
		return "", false, nil
	}
	return item.destination, true, nil
}

func (m *MemoryStore) Put(_ context.Context, url, destination string, expires time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[url] = memoryItem{destination: destination, expires: expires}
	return nil
}
