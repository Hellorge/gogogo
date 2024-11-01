package cache

import (
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/tidwall/btree"
)

const (
	shardCount     = 256
	defaultMaxSize = 10000
)

type CacheEntry struct {
	Key        string
	Value      []byte
	Expiry     time.Time
	Frequency  uint32 // For LFU eviction
	LastAccess int64  // For LRU eviction
}

type Shard struct {
	items    *btree.BTree
	lock     sync.RWMutex
	maxItems int
}

type Cache struct {
	shards  [shardCount]*Shard
	maxSize int
}

func NewCache(maxSize int) *Cache {
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}

	cache := &Cache{
		maxSize: maxSize,
	}

	// Initialize shards with BTree for each
	for i := 0; i < shardCount; i++ {
		cache.shards[i] = &Shard{
			items: btree.New(func(a, b interface{}) bool {
				return a.(CacheEntry).Frequency > b.(CacheEntry).Frequency
			}),
			maxItems: maxSize / shardCount,
		}
	}

	return cache
}

func (c *Cache) Get(key string) ([]byte, bool) {
	shard := c.shards[c.shardIndex(key)]
	shard.lock.RLock()
	item := shard.items.Get(CacheEntry{Key: key})
	if item == nil {
		shard.lock.RUnlock()
		return nil, false
	}

	entry := item.(CacheEntry)
	now := time.Now()

	// If expired, clean up asynchronously and return not found
	if now.After(entry.Expiry) {
		shard.lock.RUnlock()
		go c.cleanupEntry(shard, entry)
		return nil, false
	}

	// Fast path - just return the value
	value := entry.Value
	shard.lock.RUnlock()

	// Async update for frequency and last access
	go c.updateEntryStats(shard, entry)

	return value, true
}

func (c *Cache) cleanupEntry(shard *Shard, entry CacheEntry) {
	shard.lock.Lock()
	// Double check if it's still the same entry and still expired
	if current := shard.items.Get(CacheEntry{Key: entry.Key}); current != nil {
		currentEntry := current.(CacheEntry)
		if currentEntry.Expiry == entry.Expiry {
			shard.items.Delete(entry)
		}
	}
	shard.lock.Unlock()
}

func (c *Cache) updateEntryStats(shard *Shard, entry CacheEntry) {
	shard.lock.Lock()
	// Double check if entry still exists and update its stats
	if current := shard.items.Get(CacheEntry{Key: entry.Key}); current != nil {
		currentEntry := current.(CacheEntry)
		if currentEntry.Expiry == entry.Expiry {
			currentEntry.Frequency++
			currentEntry.LastAccess = time.Now().UnixNano()
			shard.items.Set(currentEntry)
		}
	}
	shard.lock.Unlock()
}

func (c *Cache) Set(key string, value []byte, expiry time.Time) {
	shard := c.shards[c.shardIndex(key)]
	shard.lock.Lock()
	defer shard.lock.Unlock()

	// Create new entry
	entry := CacheEntry{
		Key:        key,
		Value:      value,
		Expiry:     expiry,
		Frequency:  1,
		LastAccess: time.Now().UnixNano(),
	}

	// Check if we need to evict
	if shard.items.Len() >= shard.maxItems {
		c.evict(shard)
	}

	shard.items.Set(entry)
}

func (c *Cache) shardIndex(key string) uint64 {
	return xxhash.Sum64String(key) % shardCount
}

func (c *Cache) evict(shard *Shard) {
	now := time.Now()
	nowNano := now.UnixNano()

	// Pre-allocate with expected capacity
	maxEvict := shard.items.Len() / 10
	toDelete := make([]interface{}, 0, maxEvict)

	shard.items.Ascend(CacheEntry{}, func(item interface{}) bool {
		entry := item.(CacheEntry)
		if nowNano-entry.LastAccess > int64(time.Hour) ||
			entry.Frequency == 1 ||
			now.After(entry.Expiry) {
			toDelete = append(toDelete, item)
		}
		return len(toDelete) < maxEvict
	})

	if len(toDelete) > 0 {
		for _, item := range toDelete {
			shard.items.Delete(item)
		}
	}
}

// Clear removes all items from cache
func (c *Cache) Clear() {
	for i := 0; i < shardCount; i++ {
		shard := c.shards[i]
		shard.lock.Lock()
		shard.items = btree.New(func(a, b interface{}) bool {
			return a.(CacheEntry).Frequency > b.(CacheEntry).Frequency
		})
		shard.lock.Unlock()
	}
}
