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
    Frequency  uint32    // For LFU eviction
    LastAccess int64     // For LRU eviction
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
    defer shard.lock.RUnlock()

    item := shard.items.Get(CacheEntry{Key: key})
    if item == nil {
        return nil, false
    }

    entry := item.(CacheEntry)
    if time.Now().After(entry.Expiry) {
        shard.items.Delete(entry)
        return nil, false
    }

    // Update frequency and last access
    entry.Frequency++
    entry.LastAccess = time.Now().UnixNano()
    shard.items.Set(entry)

    return entry.Value, true
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

// evict removes least frequently used items
func (c *Cache) evict(shard *Shard) {
    now := time.Now().UnixNano()
    var toDelete []interface{}

    // Collect items for eviction
    shard.items.Ascend(CacheEntry{}, func(item interface{}) bool {
        entry := item.(CacheEntry)
        // Evict if:
        // 1. Not accessed in last hour
        // 2. Low frequency (accessed only once)
        // 3. Expired
        if now-entry.LastAccess > int64(time.Hour) ||
           entry.Frequency == 1 ||
           time.Now().After(entry.Expiry) {
            toDelete = append(toDelete, item)
        }
        // Evict up to 10% of items at a time
        return len(toDelete) < shard.items.Len()/10
    })

    // Delete collected items
    for _, item := range toDelete {
        shard.items.Delete(item)
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
