package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/valkey-io/valkey-go"
	"github.com/vmihailenco/msgpack/v5"
)

type Valkey struct {
	client valkey.Client
}

func valkeyClient() (Valkey, error) {
	address := getEnvStr("VALKEY_ADDRESS", "127.0.0.1:6379")
	tlsEnabled := getEnvBool("VALKEY_TLS_ENABLED", false)

	var tlsConfig *tls.Config // nil by default
	if tlsEnabled {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: false, // Validate the server's certificate
		}
	}

	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:  []string{address},
		TLSConfig:    tlsConfig,
		DisableCache: true, // ElastiCache serverless doesn't support client-side cache
	})
	if err != nil {
		return Valkey{}, wrapErr("failed to create valkey client", err)
	}

	return Valkey{client: client}, nil
}

func (v Valkey) SaveEvent(hash string, record EventRecord) error {
	bytes, err := msgpack.Marshal(record)
	if err != nil {
		return wrapErr("failed to marshal record", err)
	}

	key := fmt.Sprintf("event:%s", hash)
	cmd := v.client.B().Set().Key(key).Value(string(bytes)).Ex(time.Second * 86400).Build() // 24 hour expiry
	err = v.client.Do(context.Background(), cmd).Error()
	if err != nil {
		return wrapErr("failed to set key", err)
	}

	return nil
}

func (v Valkey) SaveURL(hash string, record URLRecord) error {
	bytes, err := msgpack.Marshal(record)
	if err != nil {
		return wrapErr("failed to marshal record", err)
	}

	key := fmt.Sprintf("url:%s", hash)
	cmd := v.client.B().Set().Key(key).Value(string(bytes)).Ex(time.Second * 604800).Build() // 7 day expiry
	err = v.client.Do(context.Background(), cmd).Error()
	if err != nil {
		return wrapErr("failed to set key", err)
	}

	return nil
}

func (v Valkey) ReadEvent(hash string) (EventRecord, error) {
	key := fmt.Sprintf("event:%s", hash)
	cmd := v.client.B().Get().Key(key).Build()
	resp := v.client.Do(context.Background(), cmd)
	if err := resp.Error(); err != nil {
		if err == valkey.Nil {
			return EventRecord{}, nil
		}
		return EventRecord{}, wrapErr("failed to execute get command", err)
	}

	bytes, err := resp.AsBytes()
	if err != nil {
		return EventRecord{}, wrapErr("failed to convert response to bytes", err)
	}

	var record EventRecord
	err = msgpack.Unmarshal(bytes, &record)
	if err != nil {
		return EventRecord{}, wrapErr("failed to unmarshal record", err)
	}

	return record, nil
}

func (v Valkey) ReadURL(hash string) (URLRecord, error) {
	key := fmt.Sprintf("url:%s", hash)
	cmd := v.client.B().Get().Key(key).Build()
	resp := v.client.Do(context.Background(), cmd)
	if err := resp.Error(); err != nil {
		if err == valkey.Nil {
			return URLRecord{}, nil
		}
		return URLRecord{}, wrapErr("failed to execute get command", err)
	}

	bytes, err := resp.AsBytes()
	if err != nil {
		return URLRecord{}, wrapErr("failed to convert response to bytes", err)
	}

	var record URLRecord
	err = msgpack.Unmarshal(bytes, &record)
	if err != nil {
		return URLRecord{}, wrapErr("failed to unmarshal record", err)
	}

	return record, nil
}

func (v Valkey) EventKeys() (mapset.Set[string], error) {
	ctx := context.Background()
	cursor := uint64(0)
	first := true
	keys := mapset.NewSet[string]()

	for cursor != 0 || first {
		first = false

		cmd := v.client.B().Scan().Cursor(cursor).Match("event:*").Build()
		resp := v.client.Do(ctx, cmd)
		if err := resp.Error(); err != nil {
			return nil, wrapErr("failed to execute scan command", err)
		}

		// Valkey returns an array of two items: next cursor and a list of keys
		items, err := resp.ToArray()
		if err != nil {
			return nil, wrapErr("failed to convert response to array", err)
		}
		if len(items) != 2 {
			return nil, wrapErr("unexpected number of items in response", nil)
		}
		cursor, err = items[0].AsUint64()
		if err != nil {
			return nil, wrapErr("failed to convert cursor to int64", err)
		}
		toAdd, err := items[1].AsStrSlice()
		if err != nil {
			return nil, wrapErr("failed to convert keys to string slice", err)
		}

		for _, key := range toAdd {
			keys.Add(key[6:]) // Strip the prefix
		}
	}

	return keys, nil
}

func (v Valkey) EventTTL(hash string) (int64, error) {
	key := fmt.Sprintf("event:%s", hash)
	cmd := v.client.B().Ttl().Key(key).Build()
	resp := v.client.Do(context.Background(), cmd)
	if err := resp.Error(); err != nil {
		return 0, wrapErr("failed to execute ttl command", err)
	}

	ttl, err := resp.AsInt64()
	if err != nil {
		return 0, wrapErr("failed to convert response to int64", err)
	}
	if ttl == -2 {
		return 0, fmt.Errorf("key does not exist")
	}
	if ttl == -1 {
		return 0, fmt.Errorf("key has no expiration")
	}

	return ttl, nil
}

func (v Valkey) Close() {
	v.client.Close()
}
