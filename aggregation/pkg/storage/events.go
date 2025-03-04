package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/georgemblack/blue-report/pkg/util"
)

type EventRecord struct {
	Type      int       `json:"type"` // 0 = post, 1 = repost, 2 = like
	URL       string    `json:"url"`
	DID       string    `json:"did"`
	Timestamp time.Time `json:"timestamp"`
	Post      string    `json:"post"` // AT URI of the post that was created/liked/reposted
}

func (s EventRecord) IsPost() bool {
	return s.Type == 0
}

func (s EventRecord) IsRepost() bool {
	return s.Type == 1
}

func (s EventRecord) IsLike() bool {
	return s.Type == 2
}

func (a AWS) ReadEvents(key string, eventBufferSize int) ([]EventRecord, error) {
	resp, err := a.s3.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(a.cfg.ReadEventsBucketName),
		Key:    aws.String(fmt.Sprintf("events/%s.json", key)),
	})
	if err != nil {
		return nil, util.WrapErr("failed to get object", err)
	}
	defer resp.Body.Close()

	// Decode JSON lines
	dec := json.NewDecoder(resp.Body)
	events := make([]EventRecord, 0, eventBufferSize)
	for {
		event := EventRecord{}
		if err := dec.Decode(&event); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, util.WrapErr("failed to decode event", err)
		}
		events = append(events, event)
	}

	return events, nil
}

func (a AWS) FlushEvents(start time.Time, events []EventRecord) error {
	// Convert events to JSON lines
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, event := range events {
		err := enc.Encode(event)
		if err != nil {
			return util.WrapErr("failed to encode event", err)
		}
	}

	// Write to S3, with timestamp in key
	key := fmt.Sprintf("events/%s.json", start.UTC().Format("2006-01-02-15-04-05"))
	_, err := a.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(a.cfg.WriteEventsBucketName),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(buf.Bytes()),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("application/json"),
	})
	if err != nil {
		return util.WrapErr("failed to put object", err)
	}

	return nil
}

// ListEventChunks lists all S3 object keys containing events after a certain time.
// Objects are named 'events/<timestamp>.json'.
func (a AWS) ListEventChunks(start, end time.Time) ([]string, error) {
	keys := make([]string, 0)

	// List objects using a list of prefixes, one for each day between 'start' and 'end', inclusive.
	// By using prefixes, we reduce the amount of 'LIST' operations, which can be costly for objects in archival storage classes.
	prefixes := make([]string, 0)
	current := start
	for !current.After(end) {
		prefixes = append(prefixes, fmt.Sprintf("events/%s", current.Format("2006-01-02")))
		current = current.AddDate(0, 0, 1)
	}

	slog.Info(fmt.Sprintf("listing objects with prefixes: %v", prefixes))

	for _, prefix := range prefixes {
		paginator := s3.NewListObjectsV2Paginator(a.s3, &s3.ListObjectsV2Input{
			Bucket: aws.String(a.cfg.ReadEventsBucketName),
			Prefix: aws.String(prefix),
		})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(context.Background())
			if err != nil {
				return nil, util.WrapErr("failed to list objects", err)
			}

			for _, obj := range page.Contents {
				keys = append(keys, *obj.Key)
			}
		}
	}

	// Filter keys to only include those after the 'start' time, and before the 'end' time.
	filtered := make([]string, 0)
	startStr := start.UTC().Format("2006-01-02-15-04-05")
	endStr := end.UTC().Format("2006-01-02-15-04-05")
	for _, key := range keys {
		// Parse timestamp from key, i.e. 'events/2021-08-01-12-00-00.json' -> '2021-08-01-12-00-00'
		key = strings.TrimPrefix(key, "events/")
		key = strings.TrimSuffix(key, ".json")

		// Compare strings with timestamps
		if key > startStr && key < endStr {
			filtered = append(filtered, key)
		}
	}

	slices.Sort(filtered)

	slog.Info("discovered chunks", "count", len(filtered), "first", keys[0], "last", keys[len(keys)-1])
	return filtered, nil
}
