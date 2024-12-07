package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"time"

	"github.com/valkey-io/valkey-go"
)

func valkeyClient() (valkey.Client, error) {
	address := getEnvStr("VALKEY_ADDRESS", "127.0.0.1:6379")
	tlsEnabled := getEnvBool("VALKEY_TLS_ENABLED", false)

	var tlsConfig *tls.Config // nil by default
	if tlsEnabled {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: false, // Validate the server's certificate
		}
	}

	return valkey.NewClient(valkey.ClientOption{
		InitAddress:  []string{address},
		TLSConfig:    tlsConfig,
		DisableCache: true, // ElastiCache serverless doesn't support client-side cache
	})
}

func save(client valkey.Client, key string, record InternalRecord) error {
	ctx := context.Background()

	bytes, err := json.Marshal(record)
	if err != nil {
		return wrapErr("failed to marshal record", err)
	}

	cmd := client.B().Set().Key(key).Value(string(bytes)).Ex(time.Second * 86400).Build() // 24 hour expiry
	err = client.Do(ctx, cmd).Error()
	if err != nil {
		return wrapErr("failed to set key", err)
	}

	return nil
}

func read(client valkey.Client, key string) (InternalRecord, error) {
	ctx := context.Background()

	cmd := client.B().Get().Key(key).Build()
	resp := client.Do(ctx, cmd)
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
