package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/valkey-io/valkey-go"
)

type Cache struct {
	client valkey.Client
}

func cacheClient() (Cache, error) {
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
		return Cache{}, wrapErr("failed to create valkey client", err)
	}

	return Cache{client: client}, nil
}

func (v Cache) Save(key string, record InternalRecord) error {
	ctx := context.Background()

	bytes, err := json.Marshal(record)
	if err != nil {
		return wrapErr("failed to marshal record", err)
	}

	cmd := v.client.B().Set().Key(key).Value(string(bytes)).Ex(time.Second * 86400).Build() // 24 hour expiry
	err = v.client.Do(ctx, cmd).Error()
	if err != nil {
		return wrapErr("failed to set key", err)
	}

	return nil
}

func (v Cache) Read(key string) (InternalRecord, error) {
	ctx := context.Background()

	cmd := v.client.B().Get().Key(key).Build()
	resp := v.client.Do(ctx, cmd)
	if err := resp.Error(); err != nil {
		return InternalRecord{}, wrapErr("failed to execute get command", err)
	}

	bytes, err := resp.AsBytes()
	if err != nil {
		return InternalRecord{}, wrapErr("failed to convert response to bytes", err)
	}

	var record InternalRecord
	err = json.Unmarshal(bytes, &record)
	if err != nil {
		return InternalRecord{}, wrapErr("failed to unmarshal record", err)
	}

	return record, nil
}

func (v Cache) Keys() (mapset.Set[string], error) {
	ctx := context.Background()
	cursor := uint64(0)
	first := true
	keys := mapset.NewSet[string]()

	for cursor != 0 || first {
		first = false

		cmd := v.client.B().Scan().Cursor(cursor).Build()
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
			keys.Add(key)
		}
	}

	return keys, nil
}

func (v Cache) Close() {
	v.client.Close()
}
