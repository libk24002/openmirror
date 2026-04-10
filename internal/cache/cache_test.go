package cache

import (
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
