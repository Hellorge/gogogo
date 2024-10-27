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
}

func (ctx *BuildContext) initialize() error {
	// Initialize components
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

func (ctx *BuildContext) processDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			ctx.errors.Add(fmt.Errorf("error accessing %s: %w", path, err))
			return nil
		}

		if info.IsDir() || shouldIgnore(dir, path) {
			return nil
		}

		relPath, err := filepath.Rel(ctx.config.Directories.Web, path)
		if err != nil {
			ctx.errors.Add(fmt.Errorf("error calculating relative path for %s: %w", path, err))
			return nil
		}

		if ctx.shouldProcess(relPath, info) {
			ctx.workerpool.Submit(WorkItem{
				Path:    path,
				RelPath: relPath,
				Info:    info,
			})
			atomic.AddInt32(&processedFiles, 1)

			if processedFiles%10 == 0 {
				log.Printf("Progress: %d/%d files processed", processedFiles, totalFiles)
			}
		}

		return nil
	})
}

func (ctx *BuildContext) shouldProcess(relPath string, info os.FileInfo) bool {
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
