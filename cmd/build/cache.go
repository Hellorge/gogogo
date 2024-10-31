package main

import (
	"bytes"
	"encoding/json"
	"gogogo/modules/router"
	"os"
	"sync"
)

type FileCache struct {
	mu    sync.RWMutex
	data  map[string]router.FileInfo
	dirty bool
}

type BuildCache struct {
	mu    sync.RWMutex
	data  map[string]BuildCacheEntry
	dirty bool
}

type BuildCacheEntry struct {
	Content  []byte
	Hash     string
	DistPath string
}

type DependencyGraph struct {
	mu    sync.RWMutex
	nodes map[string]map[string]struct{}
}

func NewFileCache() *FileCache {
	return &FileCache{
		data: make(map[string]router.FileInfo, defaultMapSize),
	}
}

func NewBuildCache() *BuildCache {
	return &BuildCache{
		data: make(map[string]BuildCacheEntry, defaultMapSize),
	}
}

func (fc *FileCache) Load(path string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if len(data) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		return decoder.Decode(&fc.data)
	}
	return nil
}

func (fc *FileCache) Save(path string) error {
	fc.mu.RLock()
	if !fc.dirty {
		fc.mu.RUnlock()
		return nil
	}
	fc.mu.RUnlock()

	fc.mu.Lock()
	defer fc.mu.Unlock()

	var buf bytes.Buffer
	buf.Grow(len(fc.data) * 100)
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(fc.data); err != nil {
		return err
	}

	return atomicWrite(path, buf.Bytes())
}

func (fc *FileCache) Get(key string) (router.FileInfo, bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	info, ok := fc.data[key]
	return info, ok
}

func (fc *FileCache) Set(key string, info router.FileInfo) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.data[key] = info
	fc.dirty = true
}

func (fc *FileCache) GetAll() map[string]router.FileInfo {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	result := make(map[string]router.FileInfo, len(fc.data))
	for k, v := range fc.data {
		result[k] = v
	}
	return result
}

// BuildCache methods
func (bc *BuildCache) Load(path string) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if len(data) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(data))
		return decoder.Decode(&bc.data)
	}
	return nil
}

func (bc *BuildCache) Save(path string) error {
	bc.mu.RLock()
	if !bc.dirty {
		bc.mu.RUnlock()
		return nil
	}
	bc.mu.RUnlock()

	bc.mu.Lock()
	defer bc.mu.Unlock()

	var buf bytes.Buffer
	buf.Grow(len(bc.data) * 200)
	encoder := json.NewEncoder(&buf)

	if err := encoder.Encode(bc.data); err != nil {
		return err
	}

	return atomicWrite(path, buf.Bytes())
}

func (bc *BuildCache) Get(key string) (BuildCacheEntry, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	entry, ok := bc.data[key]
	return entry, ok
}

func (bc *BuildCache) Set(key string, entry BuildCacheEntry) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.data[key] = entry
	bc.dirty = true
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]map[string]struct{}, defaultMapSize),
	}
}

func (dg *DependencyGraph) AddDependency(file, dependency string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	if _, exists := dg.nodes[file]; !exists {
		dg.nodes[file] = make(map[string]struct{})
	}
	dg.nodes[file][dependency] = struct{}{}
}

func (dg *DependencyGraph) GetDependents(file string) []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	dependents := make([]string, 0)
	for dependent, deps := range dg.nodes {
		if _, exists := deps[file]; exists {
			dependents = append(dependents, dependent)
		}
	}
	return dependents
}
