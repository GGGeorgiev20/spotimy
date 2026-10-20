package spotify

import (
	"sync"
	"time"
)

// cache is a small in-memory TTL cache for read responses. Oldest entries are
// evicted once it holds max items.
// ponytail: O(n) eviction scan; fine at a few hundred entries.
type cache struct {
	mu  sync.Mutex
	max int
	m   map[string]cacheEntry
}

type cacheEntry struct {
	val     []byte
	added   time.Time
	expires time.Time
}

func newCache(max int) *cache { return &cache{max: max, m: map[string]cacheEntry{}} }

func (c *cache) get(k string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.val, true
}

func (c *cache) put(k string, v []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.max {
		var oldest string
		var t time.Time
		for k, e := range c.m {
			if oldest == "" || e.added.Before(t) {
				oldest, t = k, e.added
			}
		}
		delete(c.m, oldest)
	}
	now := time.Now()
	c.m[k] = cacheEntry{v, now, now.Add(ttl)}
}

// invalidate drops every key with the given prefix (e.g. after a mutation).
func (c *cache) invalidate(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.m {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.m, k)
		}
	}
}
