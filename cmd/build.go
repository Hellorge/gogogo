package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"gogogo/modules"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/fsnotify/fsnotify"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

var (
	watch       = flag.Bool("watch", false, "Watch for file changes")
	concurrency = flag.Int("concurrency", 0, "Number of concurrent minification processes (0 for auto)")
)

type FileInfo struct {
	ModTime   time.Time `json:"ModTime"`
	DistPath  string    `json:"DistPath"`
	DependsOn []string  `json:"DependsOn"`
}

type BinaryTrieNode struct {
	Children map[string]*BinaryTrieNode
	FileInfo *FileInfo
}

type BuildCache struct {
	Content []byte
	Hash    string
}

var (
	config         = *modules.Cfg
	fileInfos      = make(map[string]FileInfo)
	buildCache     = make(map[string]BuildCache)
	ignorePatterns []string
	fileInfoPath   = filepath.Join(config.MetaDir, "build_file_info.json")
	buildCachePath = filepath.Join(config.MetaDir, "build_cache.json")
	buildTriePath  = filepath.Join(config.MetaDir, "build_trie.bin")
	projectRoot    = ".."

	toBuildDir = []string{config.ContentDir, config.StaticDir, config.CoreDir}
)

func main() {
	flag.Parse()

	if err := os.Chdir(projectRoot); err != nil {
		fmt.Printf("Error changing to project root directory: %v\n", err)
		os.Exit(1)
	}

	loadFileInfos()
	loadBuildCache()
	loadIgnorePatterns()

	if *concurrency == 0 {
		*concurrency = runtime.NumCPU()
	}

	if *watch {
		watchFiles()
	} else {
		build()
	}

	saveFileInfos()
	saveBuildCache()
}

func loadFileInfos() {
	data, err := os.ReadFile(fileInfoPath)
	if err == nil {
		json.Unmarshal(data, &fileInfos)
	}
}

func saveFileInfos() {
	data, _ := json.Marshal(fileInfos)
	os.WriteFile(fileInfoPath, data, 0644)
}

func loadBuildCache() {
	data, err := os.ReadFile(buildCachePath)
	if err == nil {
		json.Unmarshal(data, &buildCache)
	}
}

func saveBuildCache() {
	data, _ := json.Marshal(buildCache)
	os.WriteFile(buildCachePath, data, 0644)
}

func loadIgnorePatterns() {
	file, err := os.Open(filepath.Join(config.WebDir, config.BuildIgnoreFile))
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		pattern := strings.TrimSpace(scanner.Text())
		if pattern != "" && !strings.HasPrefix(pattern, "#") {
			ignorePatterns = append(ignorePatterns, pattern)
		}
	}
}

func shouldIgnore(path string) bool {
	for _, pattern := range ignorePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

func build() {
	startTime := time.Now()

	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("text/javascript", js.Minify)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, *concurrency)
	errChan := make(chan error, *concurrency)

	for _, dir := range toBuildDir {
		dir = filepath.Join(config.WebDir, dir)
		processDirectory(dir, config.DistDir, &wg, semaphore, errChan, m, true)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errorsOccurred bool
	for err := range errChan {
		errorsOccurred = true
		fmt.Println(err)
	}

	duration := time.Since(startTime)
	fmt.Printf("Build completed in %v\n", duration)

	if errorsOccurred {
		fmt.Println("Build completed with errors. Please review the output above.")
	} else {
		fmt.Println("Build completed successfully!")
	}

	buildBinaryTrie()
}

func processDirectory(sourceDir, destDir string, wg *sync.WaitGroup, semaphore chan struct{}, errChan chan<- error, m *minify.M, minifyAll bool) {
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || shouldIgnore(path) {
			return err
		}

		ext := filepath.Ext(path)
		if !minifyAll && ext != ".html" && ext != ".css" && ext != ".js" {
			return nil
		}

		// Calculate the key by removing the sourceDir prefix
		relPath, err := filepath.Rel(config.WebDir, path)
		if err != nil {
			return fmt.Errorf("error calculating relative path: %w", err)
		}
		fileInfoKey := relPath

		fileInfo, exists := fileInfos[fileInfoKey]
		if !exists || info.ModTime().After(fileInfo.ModTime) || shouldRebuildDependents(fileInfoKey) || !fileExists(fileInfo.DistPath) {
			wg.Add(1)
			go func(path, key string, info os.FileInfo) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				fileStartTime := time.Now()
				err := processFile(m, path, key, info, sourceDir, destDir, minifyAll)
				duration := time.Since(fileStartTime)

				if err != nil {
					errChan <- fmt.Errorf("error processing %s: %v (took %v)", path, err, duration)
				} else {
					fmt.Printf("Processed %s (took %v)\n", path, duration)
				}
			}(path, fileInfoKey, info)
		} else {
			fmt.Printf("Skipping unchanged file: %s\n", path)
		}

		return nil
	})

	if err != nil {
		errChan <- fmt.Errorf("error walking directory %s: %v", sourceDir, err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func processFile(m *minify.M, path, fileInfoKey string, info os.FileInfo, sourceDir, destDir string, minifyAll bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	hash := md5.Sum(content)
	hashString := hex.EncodeToString(hash[:])

	// if cached, ok := buildCache[fileInfoKey]; ok && cached.Hash == hashString {
	// 	return nil
	// }

	ext := filepath.Ext(path)
	var mimeType string
	switch ext {
	case ".html":
		mimeType = "text/html"
	case ".css":
		mimeType = "text/css"
	case ".js":
		mimeType = "text/javascript"
	}

	var minified []byte
	if minifyAll || (ext == ".html" || ext == ".css" || ext == ".js") {
		if ext == ".js" {
			result := api.Transform(string(content), api.TransformOptions{
				Loader:            api.LoaderJS,
				MinifyWhitespace:  true,
				MinifyIdentifiers: true,
				MinifySyntax:      true,
				Sourcemap:         api.SourceMapInline,
			})
			if len(result.Errors) > 0 {
				return fmt.Errorf("error minifying: %v", result.Errors)
			}
			minified = result.Code
		} else {
			var err error
			minified, err = m.Bytes(mimeType, content)
			if err != nil {
				return fmt.Errorf("error minifying: %w", err)
			}
		}
	} else {
		minified = content
	}

	minifiedHash := md5.Sum(minified)
	minifiedHashString := hex.EncodeToString(minifiedHash[:])[:8]

	baseName := strings.TrimSuffix(filepath.Base(path), ext)
	distFileName := fmt.Sprintf("%s.%s%s", baseName, minifiedHashString, ext)
	relPath, _ := filepath.Rel(sourceDir, filepath.Dir(path))
	distPath := filepath.Join(destDir, relPath, distFileName)

	if err := os.MkdirAll(filepath.Dir(distPath), os.ModePerm); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	if err := os.WriteFile(distPath, minified, 0644); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	if oldInfo, exists := fileInfos[fileInfoKey]; exists && oldInfo.DistPath != distPath {
		os.Remove(oldInfo.DistPath)
	}

	fileInfos[fileInfoKey] = FileInfo{
		ModTime:   info.ModTime(),
		DistPath:  distPath,
		DependsOn: findDependencies(content),
	}
	buildCache[fileInfoKey] = BuildCache{Content: minified, Hash: hashString}

	return nil
}

func buildBinaryTrie() {
	root := &BinaryTrieNode{Children: make(map[string]*BinaryTrieNode)}

	fmt.Println("Building trie:")
	for key, info := range fileInfos {
		fmt.Printf("Inserting: %s -> %s\n", key, info.DistPath)
		insertIntoBinaryTrie(root, strings.Split(key, "/"), &info)
	}

	fmt.Printf("Trie built. Root children count: %d\n", len(root.Children))

	gob.Register(FileInfo{})

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(root); err != nil {
		fmt.Println("Error encoding binary trie:", err)
		os.Exit(1)
	}

	err := os.MkdirAll(config.MetaDir, os.ModePerm)
	if err != nil {
		fmt.Println("Error creating directory:", err)
		return
	}

	if err := os.WriteFile(buildTriePath, buf.Bytes(), 0644); err != nil {
		fmt.Println("Error writing binary trie:", err)
		os.Exit(1)
	}

	fmt.Printf("Binary trie written to %s. Size: %d bytes\n", buildTriePath, buf.Len())
}

func insertIntoBinaryTrie(node *BinaryTrieNode, pathParts []string, info *FileInfo) {
	if len(pathParts) == 0 {
		node.FileInfo = info
		return
	}
	part := pathParts[0]
	if _, exists := node.Children[part]; !exists {
		node.Children[part] = &BinaryTrieNode{Children: make(map[string]*BinaryTrieNode)}
	}
	insertIntoBinaryTrie(node.Children[part], pathParts[1:], info)
}

func shouldRebuildDependents(path string) bool {
	for _, info := range fileInfos {
		for _, dep := range info.DependsOn {
			if dep == path {
				return true
			}
		}
	}
	return false
}

func findDependencies(content []byte) []string {
	var deps []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, "import") || strings.Contains(line, "require") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				dep := strings.Trim(parts[len(parts)-1], `"';`)
				deps = append(deps, dep)
			}
		}
	}
	return deps
}

func watchFiles() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("Error creating watcher:", err)
		os.Exit(1)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					fmt.Println("Modified file:", event.Name)
					build()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Println("Error:", err)
			}
		}
	}()

	for _, dir := range toBuildDir {
		err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return watcher.Add(path)
			}
			return nil
		})
		if err != nil {
			fmt.Println("Error walking directory:", err)
			os.Exit(1)
		}
	}

	build()
	fmt.Println("Watching for file changes. Press Ctrl+C to stop.")
	<-done
}
