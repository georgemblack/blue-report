package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/valkey-io/valkey-go"
	"github.com/vmihailenco/msgpack/v5"
)

type Valkey struct {
	client valkey.Client
}

func New() (Valkey, error) {
	address := util.GetEnvStr("VALKEY_ADDRESS", "127.0.0.1:6379")
	tlsEnabled := util.GetEnvBool("VALKEY_TLS_ENABLED", false)

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
		return Valkey{}, util.WrapErr("failed to create valkey client", err)
	}

	return Valkey{client: client}, nil
}

func (v Valkey) SaveURL(hash string, url CacheURLRecord) error {
	bytes, err := msgpack.Marshal(url)
	if err != nil {
		return util.WrapErr("failed to marshal record", err)
	}

	key := fmt.Sprintf("url:%s", hash)
	cmd := v.client.B().Set().Key(key).Value(string(bytes)).Ex(time.Second * 604800).Build() // 7 day expiry
	err = v.client.Do(context.Background(), cmd).Error()
	if err != nil {
		return util.WrapErr("failed to set key", err)
	}

	return nil
}

func (v Valkey) ReadURL(hash string) (CacheURLRecord, error) {
	key := fmt.Sprintf("url:%s", hash)
	cmd := v.client.B().Get().Key(key).Build()
	resp := v.client.Do(context.Background(), cmd)
	if err := resp.Error(); err != nil {
		if err == valkey.Nil {
			return CacheURLRecord{}, nil
		}
		return CacheURLRecord{}, util.WrapErr("failed to execute get command", err)
	}

	bytes, err := resp.AsBytes()
	if err != nil {
		return CacheURLRecord{}, util.WrapErr("failed to convert response to bytes", err)
	}

	var record CacheURLRecord
	err = msgpack.Unmarshal(bytes, &record)
	if err != nil {
		return CacheURLRecord{}, util.WrapErr("failed to unmarshal record", err)
	}

	return record, nil
}

func (v Valkey) SavePost(hash string, post CachePostRecord) error {
	bytes, err := msgpack.Marshal(post)
	if err != nil {
		return util.WrapErr("failed to marshal record", err)
	}

	key := fmt.Sprintf("post:%s", hash)
	cmd := v.client.B().Set().Key(key).Value(string(bytes)).Ex(time.Hour * 168).Build() // 7 day expiry
	err = v.client.Do(context.Background(), cmd).Error()
	if err != nil {
		return util.WrapErr("failed to set key", err)
	}

	return nil
}

func (v Valkey) ReadPost(hash string) (CachePostRecord, error) {
	key := fmt.Sprintf("post:%s", hash)
	cmd := v.client.B().Get().Key(key).Build()
	resp := v.client.Do(context.Background(), cmd)
	if err := resp.Error(); err != nil {
		if err == valkey.Nil {
			return CachePostRecord{}, nil
		}
		return CachePostRecord{}, util.WrapErr("failed to execute get command", err)
	}

	bytes, err := resp.AsBytes()
	if err != nil {
		return CachePostRecord{}, util.WrapErr("failed to convert response to bytes", err)
	}

	var record CachePostRecord
	err = msgpack.Unmarshal(bytes, &record)
	if err != nil {
		return CachePostRecord{}, util.WrapErr("failed to unmarshal record", err)
	}

	return record, nil
}

func (v Valkey) Close() {
	v.client.Close()
}

type CacheURLRecord struct {
	URL      string `msgpack:"u"`
	Title    string `msgpack:"t"`
	ImageURL string `msgpack:"p"`
}

func (r CacheURLRecord) MissingURL() bool {
	return r.URL == ""
}

func (r CacheURLRecord) MissingFields() bool {
	if (r.URL == "") || (r.Title == "") || (r.ImageURL == "") {
		return true
	}
	return false
}

func (r CacheURLRecord) Complete() bool {
	return !r.MissingFields()
}

type CachePostRecord struct {
	URL string `msgpack:"u"`
}

func (p CachePostRecord) Valid() bool {
	return p.URL != ""
}
