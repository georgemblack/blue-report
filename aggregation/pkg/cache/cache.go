package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/georgemblack/blue-report/pkg/config"
	"github.com/georgemblack/blue-report/pkg/util"
	"github.com/valkey-io/valkey-go"
	"github.com/vmihailenco/msgpack/v5"
)

// TTLSeconds is the default time-to-live for all post and URL records in the cache.
// If a record is 'refreshed', the TTL is reset to this value.
const TTLSeconds = 43200 // 12 hours

type Valkey struct {
	client valkey.Client
}

// New creates a new Valkey client.
func New(cfg config.Config) (Valkey, error) {
	var tlsConfig *tls.Config // nil by default
	if cfg.ValkeyTLSEnabled {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: false, // Validate the server's certificate
		}
	}

	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{cfg.ValkeyAddress},
		TLSConfig:   tlsConfig,
	})
	if err != nil {
		return Valkey{}, util.WrapErr("failed to create valkey client", err)
	}

	return Valkey{client: client}, nil
}

// SaveURL saves a URL record to the cache.
func (v Valkey) SaveURL(hash string, url URLRecord) error {
	bytes, err := msgpack.Marshal(url)
	if err != nil {
		return util.WrapErr("failed to marshal record", err)
	}

	key := fmt.Sprintf("url:%s", hash)
	cmd := v.client.B().Set().Key(key).Value(string(bytes)).Ex(time.Second * TTLSeconds).Build()
	err = v.client.Do(context.Background(), cmd).Error()
	if err != nil {
		return util.WrapErr("failed to set key", err)
	}

	return nil
}

// ReadURL reads a URL record from the cache. If the record does not exist, return an empty record.
func (v Valkey) ReadURL(hash string) (URLRecord, error) {
	key := fmt.Sprintf("url:%s", hash)
	cmd := v.client.B().Get().Key(key).Build()
	resp := v.client.Do(context.Background(), cmd)
	if err := resp.Error(); err != nil {
		if err == valkey.Nil {
			return URLRecord{}, nil
		}
		return URLRecord{}, util.WrapErr("failed to execute get command", err)
	}

	bytes, err := resp.AsBytes()
	if err != nil {
		return URLRecord{}, util.WrapErr("failed to convert response to bytes", err)
	}

	var record URLRecord
	err = msgpack.Unmarshal(bytes, &record)
	if err != nil {
		return URLRecord{}, util.WrapErr("failed to unmarshal record", err)
	}

	return record, nil
}

// RefreshURL refreshes the TTL of a URL record in the cache.
func (v Valkey) RefreshURL(hash string) error {
	key := fmt.Sprintf("url:%s", hash)
	cmd := v.client.B().Expire().Key(key).Seconds(TTLSeconds).Build()
	err := v.client.Do(context.Background(), cmd).Error()
	if err != nil {
		return util.WrapErr("failed to expire key", err)
	}

	return nil
}

// SavePost saves a post record to the cache.
func (v Valkey) SavePost(hash string, post PostRecord) error {
	bytes, err := msgpack.Marshal(post)
	if err != nil {
		return util.WrapErr("failed to marshal record", err)
	}

	key := fmt.Sprintf("post:%s", hash)
	cmd := v.client.B().Set().Key(key).Value(string(bytes)).Ex(time.Second * TTLSeconds).Build()
	err = v.client.Do(context.Background(), cmd).Error()
	if err != nil {
		return util.WrapErr("failed to set key", err)
	}

	return nil
}

// ReadPost reads a post record from the cache. If the record does not exist, return an empty record.
func (v Valkey) ReadPost(hash string) (PostRecord, error) {
	key := fmt.Sprintf("post:%s", hash)
	cmd := v.client.B().Get().Key(key).Build()
	resp := v.client.Do(context.Background(), cmd)
	if err := resp.Error(); err != nil {
		if err == valkey.Nil {
			return PostRecord{}, nil
		}
		return PostRecord{}, util.WrapErr("failed to execute get command", err)
	}

	bytes, err := resp.AsBytes()
	if err != nil {
		return PostRecord{}, util.WrapErr("failed to convert response to bytes", err)
	}

	var record PostRecord
	err = msgpack.Unmarshal(bytes, &record)
	if err != nil {
		return PostRecord{}, util.WrapErr("failed to unmarshal record", err)
	}

	return record, nil
}

// RefreshPost refreshes the TTL of a post record in the cache.
func (v Valkey) RefreshPost(hash string) error {
	key := fmt.Sprintf("post:%s", hash)
	cmd := v.client.B().Expire().Key(key).Seconds(TTLSeconds).Build()
	err := v.client.Do(context.Background(), cmd).Error()
	if err != nil {
		return util.WrapErr("failed to expire key", err)
	}

	return nil
}

func (v Valkey) Close() {
	v.client.Close()
}

type URLRecord struct {
	Title      string `msgpack:"t"`
	ImageURL   string `msgpack:"p"` // Bluesky CDN URL of the thumbnail
	Totals     Totals `msgpack:"m"`
	Normalized bool   `msgpack:"n"`
}

type Totals struct {
	Posts   int `msgpack:"p"`
	Reposts int `msgpack:"r"`
	Likes   int `msgpack:"l"`
}

func (r URLRecord) TotalInteractions() int {
	return r.Totals.Posts + r.Totals.Reposts + r.Totals.Likes
}

func (r URLRecord) MissingFields() bool {
	if (r.Title == "") || (r.ImageURL == "") {
		return true
	}
	return false
}

func (r URLRecord) Complete() bool {
	return !r.MissingFields()
}

type PostRecord struct {
	URL string `msgpack:"u"`
}

func (p PostRecord) Valid() bool {
	return p.URL != ""
}
