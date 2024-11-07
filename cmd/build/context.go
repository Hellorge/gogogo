package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"gogogo/modules/config"
	"gogogo/modules/router"

	"github.com/BurntSushi/toml"
)

type BuildContext struct {
	config      *config.Config
	concurrency int
	fileCache   *FileCache
	buildCache  *BuildCache
	depGraph    *DependencyGraph
	bufferPool  *BufferPool
	workerpool  *WorkerPool
	minifier    *MinificationWorker
	errors      *ErrorCollector
	aliasMap    map[string]string
	usedAliases map[string]string
	force       bool
	target      string
	dryRun      bool
	stats       bool
	outputDir   string
}

type MetaData struct {
	Alias string `toml:"alias"`
}

func (ctx *BuildContext) initialize() error {
	// Initialize components
	ctx.aliasMap = make(map[string]string)
	ctx.usedAliases = make(map[string]string)

	ctx.workerpool = NewWorkerPool(ctx.concurrency, ctx)
	ctx.minifier = NewMinificationWorker()

	// Load caches concurrently
	errs := make(chan error, 2)
	go func() {
		errs <- ctx.fileCache.Load(fileInfoPath)
	}()
	go func() {
		errs <- ctx.buildCache.Load(buildCachePath)
	}()

	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			return fmt.Errorf("failed to load caches: %w", err)
		}
	}

	return nil
}

func (ctx *BuildContext) build() error {
	log.Println("Starting build process...")

	// Count total files first
	for _, dir := range toBuildDir {
		filepath.Walk(dir,
			func(_ string, info os.FileInfo, _ error) error {
				if info != nil && !info.IsDir() {
					atomic.AddInt32(&totalFiles, 1)
				}
				return nil
			})
	}

	// Process directories
	for _, dir := range toBuildDir {
		if err := ctx.processDirectory(dir); err != nil {
			return err
		}
	}

	// Wait for completion
	if err := ctx.workerpool.Wait(); err != nil {
		return err
	}

	// Build router binary
	if err := ctx.buildRouterBinary(); err != nil {
		return err
	}

	// Save caches
	if err := ctx.saveAllCaches(); err != nil {
		return err
	}

	if ctx.errors.HasErrors() {
		return ctx.errors.Error()
	}

	return nil
}

func (ctx *BuildContext) saveAllCaches() error {
	errs := make(chan error, 2)
	go func() {
		errs <- ctx.fileCache.Save(fileInfoPath)
	}()
	go func() {
		errs <- ctx.buildCache.Save(buildCachePath)
	}()

	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			return fmt.Errorf("failed to save caches: %w", err)
		}
	}
	return nil
}

func (ctx *BuildContext) getAliasedPath(path string) string {
	if path == "." || path == "" {
		return path
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Use the alias if it exists, otherwise use the original name
	if alias, exists := ctx.aliasMap[path]; exists {
		base = alias
	}

	parentPath := ctx.getAliasedPath(dir)
	if parentPath == "." {
		return base
	}

	return filepath.Join(parentPath, base)
}

func (ctx *BuildContext) processAlias(path string) error {
	metaPath := filepath.Join(path, "meta.toml")
	relPath, err := filepath.Rel(ctx.config.Directories.Web, path)
	if err != nil {
		return fmt.Errorf("error calculating relative path for %s: %w", path, err)
	}

	if _, err := os.Stat(metaPath); err == nil {
		data, err := os.ReadFile(metaPath)
		if err != nil {
			return fmt.Errorf("error reading meta.toml at %s: %w", path, err)
		}

		meta := &MetaData{}
		if err := toml.Unmarshal(data, meta); err != nil {
			return fmt.Errorf("error parsing meta.toml at %s: %w", path, err)
		}

		if meta.Alias != "" {
			if strings.ContainsAny(meta.Alias, "<>:\"\\|?*") {
				return fmt.Errorf("invalid characters in alias for path %q: %q", relPath, meta.Alias)
			}

			if len(meta.Alias) > 100 {
				return fmt.Errorf("alias too long for path %q: %q (max 100 characters)", relPath, meta.Alias)
			}

			if existing, exists := ctx.usedAliases[meta.Alias]; exists {
				return fmt.Errorf("duplicate alias detected:\n"+
					"  Alias: %s\n"+
					"  Path: %s\n"+
					"  Conflicts with: %s\n",
					meta.Alias, relPath, existing)
			}

			ctx.aliasMap[relPath] = meta.Alias
			ctx.usedAliases[meta.Alias] = relPath
		}
	}
	return nil
}

func (ctx *BuildContext) processDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			ctx.errors.Add(fmt.Errorf("error accessing %s: %w", path, err))
			return nil
		}

		if info.IsDir() || shouldIgnore(dir, path) {
			if info.IsDir() {
				ctx.processAlias(path)
			}
			return nil
		}

		relPath, err := filepath.Rel(ctx.config.Directories.Web, path)
		if err != nil {
			ctx.errors.Add(fmt.Errorf("error calculating relative path for %s: %w", path, err))
			return nil
		}

		if ctx.shouldProcess(relPath, info) {
			ctx.workerpool.Submit(WorkItem{
				Path:        path,
				RelPath:     relPath,
				Info:        info,
				AliasedPath: ctx.getAliasedPath(relPath),
			})
			atomic.AddInt32(&processedFiles, 1)

			log.Printf("%d/%d %s", processedFiles, totalFiles, relPath)
		}

		return nil
	})
}

func (ctx *BuildContext) shouldProcess(relPath string, info os.FileInfo) bool {
	if ctx.force {
		return true
	}

	fileInfo, exists := ctx.fileCache.Get(relPath)
	if !exists {
		return true
	}

	if info.ModTime().After(fileInfo.ModTime) {
		return true
	}

	dependents := ctx.depGraph.GetDependents(relPath)
	return len(dependents) > 0
}

func (ctx *BuildContext) buildRouterBinary() error {
	root := &router.RadixNode{
		Children: make([]*router.RadixNode, 0, 16),
	}

	fileInfos := ctx.fileCache.GetAll()
	for path, info := range fileInfos {
		segments := strings.Split(strings.Trim(path, "/"), "/")
		root.Insert(segments, &info) // Note: need to make Insert public in RadixNode
	}

	// Create meta directory if needed
	if err := os.MkdirAll(ctx.config.Directories.Meta, 0755); err != nil {
		return err
	}

	buf := ctx.bufferPool.Get()
	defer ctx.bufferPool.Put(buf)

	buffer := bytes.NewBuffer(buf)
	buffer.Grow(1 << 20)

	if err := gob.NewEncoder(buffer).Encode(root); err != nil {
		return err
	}

	return atomicWrite(
		filepath.Join(ctx.config.Directories.Meta, "router_binary.bin"),
		buffer.Bytes(),
	)
}
