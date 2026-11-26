// Package eventlog stores events as Parquet files, in the layout described
// in docs/event-schema.md, and finds where to resume reading after a restart.
package eventlog

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress/zstd"

	"github.com/georgemblack/blue-report/backend/internal/event"
)

// SchemaVersion is saved in each file's metadata. Increase it if the
// columns ever change.
const SchemaVersion = "1"

// Prefix is where event files live in the store.
const Prefix = "events/"

// row is one row in a file. The column names and types are part of the
// stored format (docs/event-schema.md), so don't change them lightly.
type row struct {
	TimeUS     int64  `parquet:"time_us"`
	Type       int8   `parquet:"type"`
	DID        string `parquet:"did,dict"`
	RKey       string `parquet:"rkey"`
	SubjectURI string `parquet:"subject_uri,dict"`
	URL        string `parquet:"url,dict"`
}

// Key returns where a file whose newest event is at maxTimeUS is stored:
// events/date=YYYY-MM-DD/hour=HH/<maxTimeUS>.parquet, in UTC.
func Key(maxTimeUS int64) string {
	t := time.UnixMicro(maxTimeUS).UTC()
	return fmt.Sprintf("%sdate=%s/hour=%02d/%d.parquet", Prefix, t.Format(time.DateOnly), t.Hour(), maxTimeUS)
}

// timeFromKey returns the newest event time in a file, from its name.
func timeFromKey(key string) (int64, bool) {
	name := key[strings.LastIndex(key, "/")+1:]
	name, ok := strings.CutSuffix(name, ".parquet")
	if !ok {
		return 0, false
	}
	t, err := strconv.ParseInt(name, 10, 64)
	return t, err == nil
}

// Encode turns events into a Parquet file (zstd compressed). It returns the
// file and the newest event time, which names the file.
func Encode(events []event.Event) (data []byte, maxTimeUS int64, err error) {
	if len(events) == 0 {
		return nil, 0, errors.New("eventlog: no events to encode")
	}
	minTimeUS := events[0].TimeUS
	rows := make([]row, len(events))
	for i, e := range events {
		rows[i] = row{TimeUS: e.TimeUS, Type: int8(e.Type), DID: e.DID, RKey: e.RKey, SubjectURI: e.SubjectURI, URL: e.URL}
		minTimeUS = min(minTimeUS, e.TimeUS)
		maxTimeUS = max(maxTimeUS, e.TimeUS)
	}

	var buf bytes.Buffer
	w := parquet.NewGenericWriter[row](&buf,
		parquet.Compression(&zstd.Codec{}),
		parquet.KeyValueMetadata("schema_version", SchemaVersion),
		parquet.KeyValueMetadata("min_time_us", strconv.FormatInt(minTimeUS, 10)),
		parquet.KeyValueMetadata("max_time_us", strconv.FormatInt(maxTimeUS, 10)),
	)
	if _, err := w.Write(rows); err != nil {
		return nil, 0, fmt.Errorf("eventlog: encoding: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, 0, fmt.Errorf("eventlog: encoding: %w", err)
	}
	return buf.Bytes(), maxTimeUS, nil
}

// Decode reads every event in a file.
func Decode(data []byte) ([]event.Event, error) {
	rows, err := parquet.Read[row](bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("eventlog: decoding: %w", err)
	}
	events := make([]event.Event, len(rows))
	for i, r := range rows {
		events[i] = event.Event{TimeUS: r.TimeUS, Type: event.Type(r.Type), DID: r.DID, RKey: r.RKey, SubjectURI: r.SubjectURI, URL: r.URL}
	}
	return events, nil
}

// Metadata reads a file's key-value metadata (schema_version, min_time_us,
// max_time_us).
func Metadata(data []byte) (map[string]string, error) {
	f, err := parquet.OpenFile(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("eventlog: opening file: %w", err)
	}
	md := make(map[string]string)
	for _, kv := range f.Metadata().KeyValueMetadata {
		md[kv.Key] = kv.Value
	}
	return md, nil
}
