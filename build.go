package main

import (
    "bufio"
    "crypto/md5"
    "encoding/hex"
    "encoding/json"
    "flag"
    "fmt"
    "io/ioutil"
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
    watch = flag.Bool("watch", false, "Watch for file changes")
    concurrency = flag.Int("concurrency", 0, "Number of concurrent minification processes (0 for auto)")
)

type FileInfo struct {
    ModTime time.Time
    DistPath string
    DependsOn []string
}

type BuildCache struct {
    Content []byte
    Hash string
}

var (
    fileInfos = make(map[string]FileInfo)
    buildCache = make(map[string]BuildCache)
    ignorePatterns []string
    fileInfoPath = "build_file_info.json"
    buildCachePath = "build_cache.json"
    ignoreFilePath = ".buildignore"
)

func main() {
    flag.Parse()

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
    data, err := ioutil.ReadFile(fileInfoPath)
    if err == nil {
        json.Unmarshal(data, &fileInfos)
    }
}

func saveFileInfos() {
    data, _ := json.Marshal(fileInfos)
    ioutil.WriteFile(fileInfoPath, data, 0644)
}

func loadBuildCache() {
    data, err := ioutil.ReadFile(buildCachePath)
    if err == nil {
        json.Unmarshal(data, &buildCache)
    }
}

func saveBuildCache() {
    data, _ := json.Marshal(buildCache)
    ioutil.WriteFile(buildCachePath, data, 0644)
}

func loadIgnorePatterns() {
    file, err := os.Open(ignoreFilePath)
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

    err := filepath.Walk("server", func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() || shouldIgnore(path) {
            return err
        }

        ext := filepath.Ext(path)
        if ext != ".html" && ext != ".css" && ext != ".js" {
            return nil
        }

        fileInfo, exists := fileInfos[path]
        if !exists || info.ModTime().After(fileInfo.ModTime) || shouldRebuildDependents(path) {
            wg.Add(1)
            go func(path string, info os.FileInfo) {
                defer wg.Done()
                semaphore <- struct{}{}
                defer func() { <-semaphore }()

                fileStartTime := time.Now()
                err := processFile(m, path, info)
                duration := time.Since(fileStartTime)

                if err != nil {
                    errChan <- fmt.Errorf("error processing %s: %v (took %v)", path, err, duration)
                } else {
                    fmt.Printf("Processed %s (took %v)\n", path, duration)
                }
            }(path, info)
        } else {
            fmt.Printf("Skipping unchanged file: %s\n", path)
        }

        return nil
    })

    go func() {
        wg.Wait()
        close(errChan)
    }()

    var errorsOccurred bool
    for err := range errChan {
        errorsOccurred = true
        fmt.Println(err)
    }

    if err != nil {
        fmt.Println("Error during build:", err)
        os.Exit(1)
    }

    duration := time.Since(startTime)
    fmt.Printf("Build completed in %v\n", duration)

    if errorsOccurred {
        fmt.Println("Build completed with errors. Please review the output above.")
    } else {
        fmt.Println("Build completed successfully!")
    }
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

func processFile(m *minify.M, path string, info os.FileInfo) error {
    content, err := ioutil.ReadFile(path)
    if err != nil {
        return fmt.Errorf("error reading file: %w", err)
    }

    hash := md5.Sum(content)
    hashString := hex.EncodeToString(hash[:])

    if cached, ok := buildCache[path]; ok && cached.Hash == hashString {
        // File hasn't changed, no need to process
        return nil
    }

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

    minifiedHash := md5.Sum(minified)
    minifiedHashString := hex.EncodeToString(minifiedHash[:])[:8]

    baseName := strings.TrimSuffix(filepath.Base(path), ext)
    distFileName := fmt.Sprintf("%s.%s%s", baseName, minifiedHashString, ext)
    distPath := filepath.Join("dist", strings.TrimPrefix(filepath.Dir(path), "server"), distFileName)

    if err := os.MkdirAll(filepath.Dir(distPath), os.ModePerm); err != nil {
        return fmt.Errorf("error creating directory: %w", err)
    }

    if err := ioutil.WriteFile(distPath, minified, 0644); err != nil {
        return fmt.Errorf("error writing file: %w", err)
    }

    if oldInfo, exists := fileInfos[path]; exists && oldInfo.DistPath != distPath {
        os.Remove(oldInfo.DistPath)
    }

    fileInfos[path] = FileInfo{
        ModTime:   info.ModTime(),
        DistPath:  distPath,
        DependsOn: findDependencies(content),
    }
    buildCache[path] = BuildCache{Content: minified, Hash: hashString}

    return nil
}

func findDependencies(content []byte) []string {
    // This is a simple implementation. You may need to adjust it based on your project's structure
    var deps []string
    lines := strings.Split(string(content), "\n")
    for _, line := range lines {
        if strings.Contains(line, "import") || strings.Contains(line, "require") {
            // Extract the file path and add it to deps
            // This is a simplified example and may need to be more robust
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

    err = filepath.Walk("server", func(path string, info os.FileInfo, err error) error {
        if info.IsDir() {
            return watcher.Add(path)
        }
        return nil
    })
    if err != nil {
        fmt.Println("Error walking directory:", err)
        os.Exit(1)
    }

    build()
    fmt.Println("Watching for file changes. Press Ctrl+C to stop.")
    <-done
}