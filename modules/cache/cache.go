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
	Frequency  uint32
	LastAccess int64
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

func (ac *Cache) Get(key string) ([]byte, bool) {
	shard := ac.shards[ac.shardIndex(key)]
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

	entry.Frequency++
	entry.LastAccess = time.Now().UnixNano()
	shard.items.Set(entry)
	return entry.Value, true
}

func (ac *Cache) Set(key string, value []byte, expiry time.Time) {
	shard := ac.shards[ac.shardIndex(key)]
	shard.lock.Lock()
	defer shard.lock.Unlock()

	entry := CacheEntry{
		Key:        key,
		Value:      value,
		Expiry:     expiry,
		Frequency:  1,
		LastAccess: time.Now().UnixNano(),
	}

	if shard.items.Len() >= shard.maxItems {
		ac.evict(shard)
	}

	shard.items.Set(entry)
}

func (ac *Cache) shardIndex(key string) uint64 {
	return xxhash.Sum64String(key) % shardCount
}

func (ac *Cache) evict(shard *Shard) {
	now := time.Now().UnixNano()
	var toDelete []interface{}
	shard.items.Ascend(CacheEntry{}, func(item interface{}) bool {
		entry := item.(CacheEntry)
		if now-entry.LastAccess > int64(time.Hour) || entry.Frequency == 1 {
			toDelete = append(toDelete, item)
		}
		return len(toDelete) < shard.items.Len()/10 // Evict up to 10% of items
	})
	for _, item := range toDelete {
		shard.items.Delete(item)
	}
}
