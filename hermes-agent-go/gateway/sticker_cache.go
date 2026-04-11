package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// StickerCache deduplicates platform attachments by SHA-256 so that
// the model sees a stable hash reference instead of an ephemeral URL.
// It is intentionally small — purely in-memory, no persistence.
type StickerCache struct {
	mu    sync.RWMutex
	bytes map[string][]byte // hash -> content
	meta  map[string]string // hash -> mime type or platform-native id
}

func NewStickerCache() *StickerCache {
	return &StickerCache{
		bytes: make(map[string][]byte),
		meta:  make(map[string]string),
	}
}

// Hash returns the SHA-256 hex digest of data.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Put stores a blob keyed by its hash, returning the hash.
func (s *StickerCache) Put(data []byte, meta string) string {
	h := Hash(data)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bytes[h]; !ok {
		// Copy data so callers can't mutate the stored buffer.
		cp := make([]byte, len(data))
		copy(cp, data)
		s.bytes[h] = cp
		s.meta[h] = meta
	}
	return h
}

// Get fetches a previously stored blob.
func (s *StickerCache) Get(hash string) ([]byte, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bytes[hash]
	if !ok {
		return nil, "", false
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp, s.meta[hash], true
}

// Has reports whether a hash is already cached.
func (s *StickerCache) Has(hash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.bytes[hash]
	return ok
}

// Len returns the number of distinct cached blobs.
func (s *StickerCache) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.bytes)
}
