package feed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Entry is one link in the feed.
type Entry struct {
	Title   string    `json:"title"`
	URL     string    `json:"url"`
	Posts   []Post    `json:"recommended_posts"`
	AddedAt time.Time `json:"-"`
}

type Post struct {
	Rank     int    `json:"rank"`
	AtURI    string `json:"at_uri"`
	Username string `json:"username"`
	Handle   string `json:"handle"`
	Text     string `json:"text"`
}

// Store keeps the feed's entries.
type Store interface {
	List(ctx context.Context) ([]Entry, error)
	// Add saves an entry, unless the link is already in the feed.
	Add(ctx context.Context, e Entry, expires time.Time) error
}

// DynamoStore keeps entries in the blue-report-feed-entries table. Items
// look like v1's blue-report-feed items, so v1's entries can be copied over:
// {urlHash, timestamp (RFC 3339), content (the Entry as JSON)}, plus
// expiresAt (seconds since 1970), after which DynamoDB deletes the item.
type DynamoStore struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoStore(client *dynamodb.Client, table string) *DynamoStore {
	return &DynamoStore{client: client, table: table}
}

func (d *DynamoStore) List(ctx context.Context) ([]Entry, error) {
	var entries []Entry
	pages := dynamodb.NewScanPaginator(d.client, &dynamodb.ScanInput{TableName: aws.String(d.table)})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading feed entries: %w", err)
		}
		for _, item := range page.Items {
			e, err := fromItem(item)
			if err != nil {
				return nil, err
			}
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func fromItem(item map[string]types.AttributeValue) (Entry, error) {
	var e Entry
	content, ok := item["content"].(*types.AttributeValueMemberS)
	timestamp, ok2 := item["timestamp"].(*types.AttributeValueMemberS)
	if !ok || !ok2 {
		return e, errors.New("feed entry is missing its content or timestamp")
	}
	if err := json.Unmarshal([]byte(content.Value), &e); err != nil {
		return e, fmt.Errorf("reading feed entry: %w", err)
	}
	added, err := time.Parse(time.RFC3339, timestamp.Value)
	if err != nil {
		return e, fmt.Errorf("reading feed entry time: %w", err)
	}
	e.AddedAt = added
	return e, nil
}

func (d *DynamoStore) Add(ctx context.Context, e Entry, expires time.Time) error {
	content, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.table),
		Item: map[string]types.AttributeValue{
			"urlHash":   &types.AttributeValueMemberS{Value: urlKey(e.URL)},
			"timestamp": &types.AttributeValueMemberS{Value: e.AddedAt.UTC().Format(time.RFC3339)},
			"content":   &types.AttributeValueMemberS{Value: string(content)},
			"expiresAt": &types.AttributeValueMemberN{Value: strconv.FormatInt(expires.Unix(), 10)},
		},
		// Never replace an entry that's already there.
		ConditionExpression: aws.String("attribute_not_exists(urlHash)"),
	})
	var exists *types.ConditionalCheckFailedException
	if errors.As(err, &exists) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("adding feed entry for %s: %w", e.URL, err)
	}
	return nil
}

// urlKey names a link in the table, as v1 does.
func urlKey(url string) string {
	h := fnv.New64a()
	h.Write([]byte(url))
	return fmt.Sprintf("%x", h.Sum64())
}

// MemoryStore keeps entries in memory, for local runs and tests. Entries
// don't expire.
type MemoryStore struct {
	mu      sync.Mutex
	entries []Entry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (m *MemoryStore) List(context.Context) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Entry{}, m.entries...), nil
}

func (m *MemoryStore) Add(_ context.Context, e Entry, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.entries {
		if existing.URL == e.URL {
			return nil
		}
	}
	m.entries = append(m.entries, e)
	return nil
}
