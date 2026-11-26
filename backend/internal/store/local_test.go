package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var ctx = context.Background()

func TestPutGet(t *testing.T) {
	s := NewLocal(t.TempDir())
	if err := s.Put(ctx, "events/date=2026-11-26/hour=15/123.parquet", []byte("hello"), Meta{}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "events/date=2026-11-26/hour=15/123.parquet")
	if err != nil || string(got) != "hello" {
		t.Fatalf("Get = %q, %v", got, err)
	}

	// Overwriting replaces the file.
	if err := s.Put(ctx, "events/date=2026-11-26/hour=15/123.parquet", []byte("bye"), Meta{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, "events/date=2026-11-26/hour=15/123.parquet"); string(got) != "bye" {
		t.Fatalf("Get after overwrite = %q", got)
	}
}

func TestGetMissing(t *testing.T) {
	_, err := NewLocal(t.TempDir()).Get(ctx, "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestList(t *testing.T) {
	s := NewLocal(t.TempDir())
	keys := []string{
		"events/date=2026-11-26/hour=15/200.parquet",
		"events/date=2026-11-25/hour=23/100.parquet",
		"events/date=2026-11-26/hour=09/150.parquet",
		"data/top-links.json",
	}
	for _, k := range keys {
		if err := s.Put(ctx, k, []byte(k), Meta{}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.List(ctx, "events/date=2026-11-26/")
	want := []string{"events/date=2026-11-26/hour=09/150.parquet", "events/date=2026-11-26/hour=15/200.parquet"}
	if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("List = %v, %v; want %v", got, err, want)
	}

	all, _ := s.List(ctx, "")
	if len(all) != 4 || all[0] != "data/top-links.json" {
		t.Fatalf("List all = %v", all)
	}
}

func TestListEmptyStore(t *testing.T) {
	s := NewLocal(filepath.Join(t.TempDir(), "does-not-exist-yet"))
	got, err := s.List(ctx, "events/")
	if err != nil || len(got) != 0 {
		t.Fatalf("List = %v, %v; want nothing", got, err)
	}
}

// Leftover temporary files (from a crash mid-write) aren't listed.
func TestListSkipsTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	s := NewLocal(dir)
	if err := s.Put(ctx, "events/a.parquet", []byte("x"), Meta{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events", ".tmp-123"), []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.List(ctx, ""); fmt.Sprint(got) != "[events/a.parquet]" {
		t.Fatalf("List = %v", got)
	}
}

func TestInvalidKeys(t *testing.T) {
	s := NewLocal(t.TempDir())
	for _, key := range []string{"", "/abs", "../escape", "a/../../b"} {
		if err := s.Put(ctx, key, nil, Meta{}); err == nil {
			t.Errorf("Put(%q) succeeded, want error", key)
		}
	}
}
