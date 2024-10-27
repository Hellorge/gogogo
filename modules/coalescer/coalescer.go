package coalescer

import (
	"hash/fnv"
	"sync"
	"sync/atomic"
)

const shardCount = 32 // You can adjust this based on your needs

type Shard struct {
	sync.RWMutex
	calls map[string]*Call
}

type Call struct {
	wg     sync.WaitGroup
	val    []byte
	err    error
	loaded int32
}

type Coalescer struct {
	shards [shardCount]Shard
}
func NewCoalescer() *Coalescer {
	c := &Coalescer{}
	for i := range c.shards {
		c.shards[i].calls = make(map[string]*Call)
	}
	return c
}

func (co *Coalescer) getShard(key string) *Shard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return &co.shards[h.Sum32()%shardCount]
}

func (co *Coalescer) Do(key string, fn func() ([]byte, error)) ([]byte, error) {
	shard := co.getShard(key)
	shard.RLock()
	c, ok := shard.calls[key]
	shard.RUnlock()

	if ok {
		c.wg.Wait()
		return c.val, c.err
	}

	c = &Call{}
	c.wg.Add(1)

	shard.Lock()
	existing, loaded := shard.calls[key]
	if loaded {
		shard.Unlock()
		existing.wg.Wait()
		return existing.val, existing.err
	}
	shard.calls[key] = c
	shard.Unlock()

	c.val, c.err = fn()
	atomic.StoreInt32(&c.loaded, 1)
	c.wg.Done()

	shard.Lock()
	delete(shard.calls, key)
	shard.Unlock()

	return c.val, c.err
}
