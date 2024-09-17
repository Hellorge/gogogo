package main

import (
    "encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"
    "sync"
    "time"
)

type fileCacheEntry struct {
    content     []byte
    lastChecked time.Time
}

var (
	templates = make(map[string]*template.Template)
	templateMutex sync.RWMutex

    fileCache     = make(map[string]fileCacheEntry)
    fileCacheMutex sync.RWMutex
    cacheDuration = 5 * time.Minute

	isProduction = flag.Bool("prod", false, "Run in production mode")

	baseDir string
	staticDir string
	hash_regex string
)


type PageContent struct {
	Content template.HTML
	Style   template.CSS
	Script  template.JS
}

func main() {
	flag.Parse()

	if *isProduction {
		fmt.Println("Running in production mode")
		baseDir = "dist"
		staticDir = "dist"
		hash_regex = ".????????"
	} else {
		fmt.Println("Running in development mode")
		baseDir = "server"
		staticDir = "public"
		hash_regex = ""
	}

	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Handle all other routes
    http.HandleFunc("/api/", handleAPIRequest)
    http.HandleFunc("/", handleRequest)

	// Start the server
	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleAPIRequest(w http.ResponseWriter, r *http.Request) {
    path := strings.TrimPrefix(r.URL.Path, "/api")
    if path == "" || path == "/" {
        path = "/home"
    }
    path = strings.TrimPrefix(path, "/")

    content, err := loadContent(path)
    if err != nil {
        http.Error(w, "Page not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(content)
}


func handleRequest(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path
    if path == "/" {
        path = "/home"
    }
    path = strings.TrimPrefix(path, "/")

    content, err := loadContent(path)
    if err != nil {
        http.Error(w, "Page not found", http.StatusNotFound)
        return
    }

    if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
        // This is an AJAX request, return JSON
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(content)
    } else {
        // This is a regular request, render the full page
        tmpl, err := loadTemplate("index.html")
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        err = tmpl.Execute(w, content)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
        }
    }
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

    tmpl, err := template.ParseFiles(filepath.Join(staticDir, name))
    if err != nil {
        return nil, err
    }
    templates[name] = tmpl
    return tmpl, nil
}


func loadContent(path string) (PageContent, error) {
    // Construct the full path
    basePath := filepath.Join(baseDir, path)

    // Load content
    content, err := findAndReadFile(basePath, "content.html")
    if err != nil {
        return PageContent{}, fmt.Errorf("content file error: %w", err)
    }

    // Load style (if exists)
    style, _ := findAndReadFile(basePath, "style.css")

    // Load script (if exists)
    script, _ := findAndReadFile(basePath, "script.js")

    return PageContent{
        Content: template.HTML(content),
        Style:   template.CSS(style),
        Script:  template.JS(script),
    }, nil
}

func findAndReadFile(basePath, filename string) ([]byte, error) {
    cacheKey := filepath.Join(basePath, filename)

    if *isProduction {
        // Check cache first in production mode
        fileCacheMutex.RLock()
        if entry, ok := fileCache[cacheKey]; ok && time.Since(entry.lastChecked) < cacheDuration {
            fileCacheMutex.RUnlock()
            return entry.content, nil
        }
        fileCacheMutex.RUnlock()
    }

    // File not in cache or cache expired, read from disk
    ext := filepath.Ext(filename)
    name := strings.TrimSuffix(filename, ext)
    pattern := filepath.Join(basePath, name + hash_regex + ext)

    matches, err := filepath.Glob(pattern)
    if err != nil {
        log.Printf("Error searching for file %s: %v", filename, err)
        return nil, fmt.Errorf("error searching for file %s: %w", filename, err)
    }
    
    switch len(matches) {
    case 0:
        log.Printf("File not found: %s", filename)
        return nil, fmt.Errorf("file not found: %s", filename)
    case 1:
        content, err := ioutil.ReadFile(matches[0])
        if err != nil {
            log.Printf("Error reading file %s: %v", matches[0], err)
            return nil, fmt.Errorf("error reading file %s: %w", matches[0], err)
        }

        if *isProduction {
            // Update cache in production mode
            fileCacheMutex.Lock()
            fileCache[cacheKey] = fileCacheEntry{content: content, lastChecked: time.Now()}
            fileCacheMutex.Unlock()
        }

        return content, nil
    default:
        log.Printf("Multiple matches found for file: %s", filename)
        return nil, fmt.Errorf("multiple matches found for file: %s", filename)
    }
}