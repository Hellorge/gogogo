package modules

import (
	"sync"
)

type Coalescer struct {
	mu    sync.Mutex
	calls map[string]*Call
}

type Call struct {
	wg     sync.WaitGroup
	val    []byte
	err    error
	loaded bool
}

func NewCoalescer() *Coalescer {
	return &Coalescer{
		calls: make(map[string]*Call),
	}
}

func (co *Coalescer) Do(key string, fn func() ([]byte, error)) ([]byte, error) {
	co.mu.Lock()
	if c, ok := co.calls[key]; ok {
		co.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}

	c := &Call{}
	c.wg.Add(1)
	co.calls[key] = c
	co.mu.Unlock()

	go func() {
		c.val, c.err = fn()
		c.loaded = true
		c.wg.Done()

		co.mu.Lock()
		delete(co.calls, key)
		co.mu.Unlock()
	}()

	c.wg.Wait()
	return c.val, c.err
}
