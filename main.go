package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "html/template"
    "io/ioutil"
    "log"
    // "os"
    "net/http"
    "path/filepath"
    "strings"
    "sync"
    "time"
)

type PageContent struct {
    Content template.HTML
    Style   template.CSS
    Script  template.JS
    IsSPAMode   bool
}

type fileCacheEntry struct {
    content     []byte
    lastChecked time.Time
}

type FileInfo struct {
    ModTime   time.Time `json:"ModTime"`
    DistPath  string    `json:"DistPath"`
    DependsOn []string  `json:"DependsOn"`
}

var (
    templateDir string
    templates     = make(map[string]*template.Template)
    templateMutex sync.RWMutex

    fileCacheMutex sync.RWMutex
    cacheDuration = 5 * time.Minute
    fileCache     = make(map[string]fileCacheEntry)
    fileInfoMap    map[string]FileInfo
    fileInfoOnce   sync.Once

    isProduction  = flag.Bool("prod", false, "Run in production mode")
    IsSPAMode     = flag.Bool("spa", false, "Run in SPA mode")

    baseDir   string
    staticDir = "public"
    hashRegex string

    findAndReadFile = fnrDev
)

const spaPrefix = "/__spa__/"

func main() {
    flag.Parse()

    if *isProduction {
        fmt.Println("Running in production mode")
        baseDir = "dist"
        hashRegex = ".????????"
        templateDir = "public"

        fileInfoOnce.Do(loadFileInfo)
        findAndReadFile = fnrDev 
    } else {
        fmt.Println("Running in development mode")
        baseDir = "server"
        templateDir = "public"
        hashRegex = ""
    }

    if _, err := loadTemplate("index.html"); err != nil {
        log.Fatalf("Failed to load main template: %v", err)
    }

    fs := http.FileServer(http.Dir(staticDir))
    http.Handle("/static/", http.StripPrefix("/static/", fs))

    http.HandleFunc("/", webHandler)
    http.HandleFunc("/api/", jsonHandler)
    http.HandleFunc(spaPrefix, jsonHandler)

    fmt.Println("Server starting on http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func webHandler(w http.ResponseWriter, r *http.Request) {
    path := normalizePath(r.URL.Path)
    content, err := loadContent(path)
    if err != nil {
        http.Error(w, "Page not found", http.StatusNotFound)
        return
    }

    tmpl, err := loadTemplate("index.html")
    if err != nil {
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }

    if err := tmpl.Execute(w, content); err != nil {
        log.Printf("Error executing template: %v", err)
    }
}

func jsonHandler(w http.ResponseWriter, r *http.Request) {
    path := strings.TrimPrefix(r.URL.Path, "/api")
    path = normalizePath(strings.TrimPrefix(path, spaPrefix))
    content, err := loadContent(path)
    if err != nil {
        http.Error(w, "Page not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(content); err != nil {
        log.Printf("Error encoding JSON: %v", err)
    }
}

func normalizePath(path string) string {
    if path == "/" || path == "" {
        return "home"
    }
    return strings.TrimPrefix(path, "/")
}

func loadTemplate(name string) (*template.Template, error) {
    templateMutex.RLock()
    tmpl, ok := templates[name]
    templateMutex.RUnlock()
    if ok {
        return tmpl, nil
    }

    templateMutex.Lock()
    defer templateMutex.Unlock()

    filePath := filepath.Join(templateDir, name)
    tmpl, err := template.ParseFiles(filePath)
    if err != nil {
        return nil, err
    }
    templates[name] = tmpl
    return tmpl, nil
}


func loadContent(path string) (PageContent, error) {
    content, err := findAndReadFile(path, "content.html")
    if err != nil {
        return PageContent{}, fmt.Errorf("content file error: %w", err)
    }

    style, _ := findAndReadFile(path, "style.css")
    script, _ := findAndReadFile(path, "script.js")

    return PageContent{
        Content: template.HTML(content),
        Style:   template.CSS(style),
        Script:  template.JS(script),
        IsSPAMode: *IsSPAMode,
    }, nil
}

func fnrDev(path, filename string) ([]byte, error) {
    // Development mode - simplified, no caching
    fullPath := filepath.Join(baseDir, path, filename)
    content, err := ioutil.ReadFile(fullPath)
    if err != nil {
        return nil, fmt.Errorf("error reading file %s: %w", fullPath, err)
    }
    return content, nil
}

func fnrProd(path, filename string) ([]byte, error) {
    cacheKey := filepath.Join("server", path, filename)

    fileCacheMutex.RLock()
    if entry, ok := fileCache[cacheKey]; ok && time.Since(entry.lastChecked) < cacheDuration {
        fileCacheMutex.RUnlock()
        return entry.content, nil
    }
    fileCacheMutex.RUnlock()

    info, ok := fileInfoMap[cacheKey]
    var fullPath = info.DistPath
    if !ok {
        return nil, fmt.Errorf("file not found in production map: %s", filename)
    }

    content, err := ioutil.ReadFile(fullPath)
    if err != nil {
        return nil, fmt.Errorf("error reading file %s: %w", fullPath, err)
    }

    fileCacheMutex.Lock()
    fileCache[cacheKey] = fileCacheEntry{content: content, lastChecked: time.Now()}
    fileCacheMutex.Unlock()

    return content, nil
}

func loadFileInfo() {
    data, err := ioutil.ReadFile("build_file_info.json")
    if err != nil {
        fmt.Printf("Error reading build_file_info.json: %v\n", err)
        return
    }

    if err := json.Unmarshal(data, &fileInfoMap); err != nil {
        fmt.Printf("Error unmarshaling build_file_info.json: %v\n", err)
    }
}

// func findAndReadFile(path, filename string) ([]byte, error) {
//     basePath := filepath.Join(baseDir, path) // dist/home/content.html
//     cacheKey := filepath.Join("server", path, filename) // server/home/content.html

//     fileCacheMutex.RLock()
//     if entry, ok := fileCache[cacheKey]; ok && time.Since(entry.lastChecked) < cacheDuration {
//         fileCacheMutex.RUnlock()
//         return entry.content, nil
//     }
//     fileCacheMutex.RUnlock()

//     var fullPath = filepath.Join(basePath, filename)

//     if *isProduction && info, ok := fileInfoMap[cacheKey]; ok {
//         fullPath = info.DistPath
//     }

//     content, err := ioutil.ReadFile(fullPath)
//     if err != nil {
//         if os.IsNotExist(err) {
//             return nil, fmt.Errorf("file not found: %s", filename)
//         }
//         return nil, fmt.Errorf("error reading file %s: %w", fullPath, err)
//     }

//     if *isProduction {
// 	    fileCacheMutex.Lock()
// 	    fileCache[cacheKey] = fileCacheEntry{content: content, lastChecked: time.Now()}
// 	    fileCacheMutex.Unlock()
//     }

//     return content, nil
// }