package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Value    []byte    `json:"value"`
	ExpireAt time.Time `json:"expire_at"`
}

type FSCache struct {
	root string
}

func NewFSCache(root string) *FSCache {
	return &FSCache{root: root}
}

func (c *FSCache) Set(key string, entry Entry) error {
	if err := os.MkdirAll(c.root, 0o755); err != nil {
		return err
	}

	body, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(c.pathForKey(key), body, 0o644)
}

func (c *FSCache) Get(key string) (Entry, bool, error) {
	body, err := os.ReadFile(c.pathForKey(key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Entry{}, false, nil
		}
		return Entry{}, false, err
	}

	var entry Entry
	if err := json.Unmarshal(body, &entry); err != nil {
		return Entry{}, false, err
	}

	now := time.Now()
	if !entry.ExpireAt.IsZero() && !entry.ExpireAt.After(now) {
		return Entry{}, false, nil
	}

	return entry, true, nil
}

func (c *FSCache) pathForKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(c.root, name+".json")
}
