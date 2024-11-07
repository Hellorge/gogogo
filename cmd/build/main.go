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
	treeFlag := flag.Bool("tree", false, "Display directory structure with aliases")
	watch := flag.Bool("watch", false, "Watch for file changes")
	concurrency := flag.Int("concurrency", runtime.NumCPU(), "Number of concurrent workers")
	verbose := flag.Bool("v", false, "Verbose output")
	forceMode := flag.Bool("force", false, "Force build all files")
	target := flag.String("target", "", "Build specific directory (relative to content dir)")
	dryRun := flag.Bool("dry-run", false, "Show what would be built without actually building")
	stats := flag.Bool("stats", false, "Show detailed build statistics")
	out := flag.String("out", "", "Custom output directory (default: dist)")
	flag.Parse()

	// Initialize logger
	log.SetFlags(0)
	if *verbose {
		log.SetFlags(log.Ltime | log.Lmicroseconds)
	}

	cfg, err := config.LoadConfig("web/config.toml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *out != "" {
		cfg.Directories.Dist = *out
	}

	// Initialize build context
	ctx := BuildContext{
		concurrency: *concurrency,
		fileCache:   NewFileCache(),
		buildCache:  NewBuildCache(),
		depGraph:    NewDependencyGraph(),
		bufferPool:  NewBufferPool(defaultBufferSize),
		errors:      NewErrorCollector(),
		force:       *forceMode,
		target:      *target,
		dryRun:      *dryRun,
		stats:       *stats,
		outputDir:   *out,
		config:      cfg,
	}

	if *concurrency <= 0 {
		*concurrency = runtime.NumCPU()
	} else if *concurrency > maxWorkers {
		*concurrency = maxWorkers
		log.Printf("Concurency set to %d", concurrency)
	}

	fileInfoPath = filepath.Join(cfg.Directories.Meta, "build_file_info.json")
	buildCachePath = filepath.Join(cfg.Directories.Meta, "build_cache.json")

	entries, err := os.ReadDir(cfg.Directories.Web)
	if err != nil {
		fmt.Errorf("failed to load read web directory: %w", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			toBuildDir = append(toBuildDir, filepath.Join(cfg.Directories.Web, entry.Name()))
		}
	}

	ctx.initialize()

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

	// Handle tree command
	if *treeFlag {
		treeCmd := NewTreeCommand()
		if err := treeCmd.Execute(cfg.Directories.Web); err != nil {
			log.Fatalf("Failed to display tree: %v", err)
		}
		return
	}
}
