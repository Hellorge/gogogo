package main

import (
	"flag"
	"fmt"
	"gogogo/modules/config"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	defaultBufferSize = 32 * 1024
	defaultMapSize    = 1000
	maxWorkers        = 8192
)

var (
	buildStartTime = time.Now()
	processedFiles int32
	totalFiles     int32
)

var (
	fileInfoPath   string
	buildCachePath string
	toBuildDir     []string
)

func main() {
	watch := flag.Bool("watch", false, "Watch for file changes")
	concurrency := flag.Int("concurrency", runtime.NumCPU(), "Number of concurrent workers")
	verbose := flag.Bool("v", false, "Verbose output")
	flag.Parse()

	// Initialize logger
	log.SetFlags(0)
	if *verbose {
		log.SetFlags(log.Ltime | log.Lmicroseconds)
	}

	// Initialize build context
	ctx, err := initializeBuildContext(*concurrency)
	if err != nil {
		log.Fatalf("Failed to initialize build context: %v", err)
	}

	// Run build or watch
	if *watch {
		log.Println("Starting watch mode...")
		if err := ctx.watchFiles(); err != nil {
			log.Fatalf("Watch failed: %v", err)
		}
	} else {
		if err := ctx.build(); err != nil {
			log.Fatalf("Build failed: %v", err)
		}
		log.Printf("Build completed in %v", time.Since(buildStartTime))
	}
}

func initializeBuildContext(concurrency int) (*BuildContext, error) {
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	} else if concurrency > maxWorkers {
		concurrency = maxWorkers
		log.Printf("Concurency set to %d", concurrency)
	}

	ctx := &BuildContext{
		concurrency: concurrency,
		fileCache:   NewFileCache(),
		buildCache:  NewBuildCache(),
		depGraph:    NewDependencyGraph(),
		bufferPool:  NewBufferPool(defaultBufferSize),
		errors:      NewErrorCollector(),
	}

	cfg, err := config.LoadConfig("web/config.toml")

	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	ctx.config = &cfg

	fileInfoPath = filepath.Join(cfg.Directories.Meta, "build_file_info.json")
	buildCachePath = filepath.Join(cfg.Directories.Meta, "build_cache.json")

	entries, err := os.ReadDir(cfg.Directories.Web)
	if err != nil {
		return nil, fmt.Errorf("failed to load read web directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			toBuildDir = append(toBuildDir, filepath.Join(cfg.Directories.Web, entry.Name()))
		}
	}

	ctx.initialize()

	return ctx, nil
}
