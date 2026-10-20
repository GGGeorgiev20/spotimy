package spotify

import (
	"sync"
	"time"
)

// cache is a small in-memory TTL cache for read responses, capped in bytes
// (playlist pages are hundreds of KB). Oldest entries are evicted first.
// ponytail: O(n) eviction scan; fine at a few hundred entries.
type cache struct {
	mu   sync.Mutex
	max  int // bytes
	size int
	m    map[string]cacheEntry
}

type cacheEntry struct {
	val     []byte
	added   time.Time
	expires time.Time
}

func newCache(maxBytes int) *cache { return &cache{max: maxBytes, m: map[string]cacheEntry{}} }

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
	if len(v) > c.max/4 {
		return // one huge response shouldn't flush everything else
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remove(k)
	for c.size+len(v) > c.max && len(c.m) > 0 {
		var oldest string
		var t time.Time
		for k, e := range c.m {
			if oldest == "" || e.added.Before(t) {
				oldest, t = k, e.added
			}
		}
		c.remove(oldest)
	}
	now := time.Now()
	c.m[k] = cacheEntry{v, now, now.Add(ttl)}
	c.size += len(v)
}

// invalidate drops every key with the given prefix (e.g. after a mutation).
func (c *cache) invalidate(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.m {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			c.remove(k)
		}
	}
}

func (c *cache) remove(k string) {
	if e, ok := c.m[k]; ok {
		c.size -= len(e.val)
		delete(c.m, k)
	}
}
