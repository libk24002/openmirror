package cache

import (
	"os"
	"testing"
	"time"
)

func TestFSCacheSetGetWithTTL(t *testing.T) {
	root := t.TempDir()
	c := NewFSCache(root)

	entry := Entry{
		Value:    []byte("value"),
		ExpireAt: time.Now().Add(time.Minute),
	}

	if err := c.Set("docker/library/alpine", entry); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, ok, err := c.Get("docker/library/alpine")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatalf("Get returned miss for existing key")
	}
	if string(got.Value) != string(entry.Value) {
		t.Fatalf("Get value = %q, want %q", string(got.Value), string(entry.Value))
	}

	expired := Entry{
		Value:    []byte("stale"),
		ExpireAt: time.Now().Add(-time.Second),
	}
	if err := c.Set("expired", expired); err != nil {
		t.Fatalf("Set expired returned error: %v", err)
	}

	stale, ok, err := c.Get("expired")
	if err != nil {
		t.Fatalf("Get expired returned error: %v", err)
	}
	if ok {
		t.Fatalf("Get expired returned hit: %+v", stale)
	}

	missing, ok, err := c.Get("missing")
	if err != nil {
		t.Fatalf("Get missing returned error: %v", err)
	}
	if ok {
		t.Fatalf("Get missing returned hit: %+v", missing)
	}
}

func TestFSCacheGetCorruptEntryReturnsMiss(t *testing.T) {
	root := t.TempDir()
	c := NewFSCache(root)

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	key := "corrupt"
	cachePath := c.pathForKey(key)
	if err := os.WriteFile(cachePath, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	entry, ok, err := c.Get(key)
	if err != nil {
		t.Fatalf("Get returned error for corrupt entry: %v", err)
	}
	if ok {
		t.Fatalf("Get returned hit for corrupt entry: %+v", entry)
	}
}
