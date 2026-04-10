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

	finalPath := c.pathForKey(key)
	dir := filepath.Dir(finalPath)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(finalPath)+".tmp-*")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(body); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, finalPath)
}

func (c *FSCache) Get(key string) (Entry, bool, error) {
	path := c.pathForKey(key)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Entry{}, false, nil
		}
		return Entry{}, false, err
	}

	var entry Entry
	if err := json.Unmarshal(body, &entry); err != nil {
		_ = os.Remove(path)
		return Entry{}, false, nil
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
