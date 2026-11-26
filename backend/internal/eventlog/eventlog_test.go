package eventlog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"

	"github.com/georgemblack/blue-report/backend/internal/event"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

var (
	ctx     = context.Background()
	discard = slog.New(slog.NewTextHandler(io.Discard, nil))
	// 2026-11-26 15:00:01.123456 UTC
	base int64 = 1795705201123456
)

func sampleEvents() []event.Event {
	return []event.Event{
		{TimeUS: base, Type: event.Post, DID: "did:plc:alice", RKey: "3kabc", SubjectURI: "at://did:plc:alice/app.bsky.feed.post/3kabc", URL: "https://example.com/a"},
		{TimeUS: base + 2_000_000, Type: event.Like, DID: "did:plc:bob", RKey: "3klike", SubjectURI: "at://did:plc:alice/app.bsky.feed.post/3kabc", URL: "https://example.com/a"},
		{TimeUS: base + 1_000_000, Type: event.Repost, DID: "did:plc:carol", RKey: "3krepost", SubjectURI: "at://did:plc:alice/app.bsky.feed.post/3kabc", URL: "https://example.com/a"},
	}
}

func TestKey(t *testing.T) {
	if got, want := Key(base), "events/date=2026-11-26/hour=15/1795705201123456.parquet"; got != want {
		t.Fatalf("Key = %q, want %q", got, want)
	}
	// Single-digit hours are zero-padded so they sort correctly.
	early := time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC).UnixMicro()
	if got := Key(early); got != fmt.Sprintf("events/date=2026-11-26/hour=03/%d.parquet", early) {
		t.Fatalf("Key = %q", got)
	}
}

func TestEncodeDecode(t *testing.T) {
	events := sampleEvents()
	data, maxTimeUS, err := Encode(events)
	if err != nil {
		t.Fatal(err)
	}
	if maxTimeUS != base+2_000_000 {
		t.Fatalf("maxTimeUS = %d", maxTimeUS)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(events) {
		t.Fatalf("round trip:\n got  %+v\n want %+v", got, events)
	}

	md, err := Metadata(data)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"schema_version": "1",
		"min_time_us":    fmt.Sprint(base),
		"max_time_us":    fmt.Sprint(base + 2_000_000),
	}
	for k, v := range want {
		if md[k] != v {
			t.Errorf("metadata %s = %q, want %q", k, md[k], v)
		}
	}
}

// The columns match docs/event-schema.md, so other tools (like DuckDB) can read them.
func TestSchema(t *testing.T) {
	data, _, err := Encode(sampleEvents())
	if err != nil {
		t.Fatal(err)
	}
	f, err := parquet.OpenFile(bytesReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"time_us":     "INT64",
		"type":        "INT32", // stored as a 32-bit int with an 8-bit "INT(8,true)" annotation
		"did":         "BYTE_ARRAY",
		"rkey":        "BYTE_ARRAY",
		"subject_uri": "BYTE_ARRAY",
		"url":         "BYTE_ARRAY",
	}
	fields := f.Schema().Fields()
	if len(fields) != len(want) {
		t.Fatalf("got %d columns, want %d", len(fields), len(want))
	}
	for _, field := range fields {
		if got := field.Type().Kind().String(); got != want[field.Name()] {
			t.Errorf("column %s is %s, want %s", field.Name(), got, want[field.Name()])
		}
	}
	for _, name := range []string{"did", "rkey", "subject_uri", "url"} {
		col, _ := f.Schema().Lookup(name)
		if got := col.Node.Type().LogicalType().String(); got != "STRING" {
			t.Errorf("column %s is annotated %q, want STRING", name, got)
		}
	}
	typeCol, _ := f.Schema().Lookup("type")
	if got := typeCol.Node.Type().LogicalType().String(); got != "INT(8,true)" {
		t.Errorf("column type is annotated %q, want INT(8,true)", got)
	}
}

func TestEncodeEmpty(t *testing.T) {
	if _, _, err := Encode(nil); err == nil {
		t.Fatal("Encode(nil) succeeded")
	}
}

func TestResumePoint(t *testing.T) {
	s := store.NewLocal(t.TempDir())
	now := time.UnixMicro(base).Add(time.Hour)

	// Nothing stored yet: start from now.
	if got, err := ResumePoint(ctx, s, now, 7*24*time.Hour); err != nil || got != 0 {
		t.Fatalf("empty store: %d, %v", got, err)
	}

	times := []int64{
		base - 26*3600*1_000_000, // yesterday
		base - 5*3600*1_000_000,  // earlier today, hour 10
		base,                     // hour 15
		base - 1_000_000,         // hour 15, older
	}
	for _, ts := range times {
		if err := s.Put(ctx, Key(ts), []byte("x"), store.Meta{}); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := ResumePoint(ctx, s, now, 7*24*time.Hour); got != base {
		t.Fatalf("ResumePoint = %d, want %d", got, base)
	}

	// The next day, with nothing written yet today, it finds yesterday's newest file.
	tomorrow := now.Add(24 * time.Hour)
	if got, _ := ResumePoint(ctx, s, tomorrow, 7*24*time.Hour); got != base {
		t.Fatalf("ResumePoint the next day = %d, want %d", got, base)
	}

	// Files older than the lookback are ignored.
	if got, _ := ResumePoint(ctx, s, now.Add(10*24*time.Hour), 7*24*time.Hour); got != 0 {
		t.Fatalf("ResumePoint after lookback = %d, want 0", got)
	}
}

func TestWriter(t *testing.T) {
	s := store.NewLocal(t.TempDir())
	w := NewWriter(s, discard)
	batches := make(chan []event.Event, 3)
	first := sampleEvents()
	second := []event.Event{{TimeUS: base + 60_000_000, Type: event.Like, DID: "did:plc:dan", RKey: "r", SubjectURI: "at://x", URL: "https://example.com/b"}}
	batches <- first
	batches <- nil // empty batches are skipped
	batches <- second
	close(batches)

	if err := w.Run(ctx, batches); err != nil {
		t.Fatal(err)
	}
	keys, _ := s.List(ctx, Prefix)
	want := []string{Key(base + 2_000_000), Key(base + 60_000_000)}
	if fmt.Sprint(keys) != fmt.Sprint(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	data, _ := s.Get(ctx, keys[1])
	if got, _ := Decode(data); fmt.Sprint(got) != fmt.Sprint(second) {
		t.Fatalf("second file = %+v", got)
	}
	if w.LastWrite().IsZero() {
		t.Fatal("LastWrite not set")
	}
}

// flakyStore fails the first n Puts.
type flakyStore struct {
	store.Store
	mu    sync.Mutex
	fails int
	tries int
}

func (f *flakyStore) Put(ctx context.Context, key string, body []byte, meta store.Meta) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tries++
	if f.tries <= f.fails {
		return errors.New("s3 is down")
	}
	return f.Store.Put(ctx, key, body, meta)
}

func TestWriterRetries(t *testing.T) {
	s := &flakyStore{Store: store.NewLocal(t.TempDir()), fails: 3}
	w := NewWriter(s, discard)
	w.retryMin, w.retryMax = time.Millisecond, 2*time.Millisecond

	batches := make(chan []event.Event, 1)
	batches <- sampleEvents()
	close(batches)
	if err := w.Run(ctx, batches); err != nil {
		t.Fatal(err)
	}
	if s.tries != 4 {
		t.Fatalf("tries = %d, want 4", s.tries)
	}
	if keys, _ := s.List(ctx, Prefix); len(keys) != 1 {
		t.Fatalf("keys = %v", keys)
	}
}

func TestWriterGivesUpWhenCancelled(t *testing.T) {
	s := &flakyStore{Store: store.NewLocal(t.TempDir()), fails: 1 << 30}
	w := NewWriter(s, discard)
	w.retryMin, w.retryMax = time.Millisecond, time.Millisecond

	batches := make(chan []event.Event, 2)
	batches <- sampleEvents()
	batches <- sampleEvents()
	close(batches)

	c, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	err := w.Run(c, batches)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if want := "stopped before saving 6 events"; !contains(err.Error(), want) {
		t.Fatalf("err = %v, want it to mention %q", err, want)
	}
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestFilesSince(t *testing.T) {
	s := store.NewLocal(t.TempDir())
	hour := int64(3600 * 1_000_000)
	times := []int64{base - 50*hour, base - 26*hour, base - 2*hour, base}
	for _, ts := range times {
		if err := s.Put(ctx, Key(ts), []byte("x"), store.Meta{}); err != nil {
			t.Fatal(err)
		}
	}
	// From 30 hours before base: skips the 50-hour-old file, crosses into yesterday.
	got, err := FilesSince(ctx, s, time.UnixMicro(base-30*hour), time.UnixMicro(base))
	want := []string{Key(base - 26*hour), Key(base - 2*hour), Key(base)}
	if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("FilesSince = %v, %v; want %v", got, err, want)
	}
}

func TestReadFiles(t *testing.T) {
	s := store.NewLocal(t.TempDir())
	var keys []string
	for i := range 20 {
		events := sampleEvents()
		for j := range events {
			events[j].TimeUS += int64(i) * 60_000_000
		}
		data, maxT, err := Encode(events)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Put(ctx, Key(maxT), data, store.Meta{}); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, Key(maxT))
	}

	seen := map[string]int{}
	err := ReadFiles(ctx, s, keys, 4, func(key string, events []event.Event) error {
		seen[key] = len(events) // no lock needed: fn is called one at a time
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 20 {
		t.Fatalf("read %d files, want 20", len(seen))
	}
	for k, n := range seen {
		if n != 3 {
			t.Fatalf("%s had %d events, want 3", k, n)
		}
	}
}

func TestReadFilesErrors(t *testing.T) {
	s := store.NewLocal(t.TempDir())
	if err := s.Put(ctx, "events/bad.parquet", []byte("not parquet"), store.Meta{}); err != nil {
		t.Fatal(err)
	}
	if err := ReadFiles(ctx, s, []string{"events/bad.parquet"}, 2, func(string, []event.Event) error { return nil }); err == nil {
		t.Fatal("corrupt file: want an error")
	}
	if err := ReadFiles(ctx, s, []string{"events/missing.parquet"}, 2, func(string, []event.Event) error { return nil }); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing file: err = %v", err)
	}

	// An error from fn stops reading.
	data, maxT, _ := Encode(sampleEvents())
	_ = s.Put(ctx, Key(maxT), data, store.Meta{})
	boom := errors.New("boom")
	if err := ReadFiles(ctx, s, []string{Key(maxT), Key(maxT), Key(maxT)}, 2, func(string, []event.Event) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("fn error: err = %v", err)
	}
}
