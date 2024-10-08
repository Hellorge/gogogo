package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gogogo/modules"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PageContent struct {
	Content   template.HTML
	Style     template.CSS
	Script    template.JS
	IsSPAMode bool
}

type ServerConfig struct {
	IsProduction  bool
	IsSPAMode     bool
	EnableMetrics bool
}

var (
	templateDir   string
	templates     = make(map[string]*template.Template)
	templateMutex sync.RWMutex

	isProduction  = flag.Bool("prod", false, "Run in production mode")
	isSPAMode     = flag.Bool("spa", false, "Run in SPA mode")
	enableMetrics = flag.Bool("metrics", false, "Enable metrics collection")

	baseDir   string
	staticDir = "public"

	findAndReadFile func(string, string) ([]byte, error)
	advancedCache   *modules.AdvancedCache
	reqCoalescer    *modules.Coalescer
	buildRouter     *modules.Router
)

const spaPrefix = "/__spa__/"

func main() {
	flag.Parse()

	config := ServerConfig{
		IsProduction:  *isProduction,
		IsSPAMode:     *isSPAMode,
		EnableMetrics: *enableMetrics,
	}

	server, err := NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	fmt.Printf("Server starting on http://localhost:8080 (Production: %v, SPA: %v, Metrics: %v)\n",
		config.IsProduction, config.IsSPAMode, config.EnableMetrics)
	log.Fatal(http.ListenAndServe(":8080", server))
}

func NewServer(config ServerConfig) (http.Handler, error) {
	mux := http.NewServeMux()

	advancedCache = modules.NewAdvancedCache(100000)
	reqCoalescer = modules.NewCoalescer()

	if config.IsProduction {
		fmt.Println("Running in production mode")
		baseDir = "dist"
		templateDir = "public"
		findAndReadFile = fnrProd

		var err error
		buildRouter, err = modules.NewRouter("build_file_info.json")
		if err != nil {
			return nil, fmt.Errorf("failed to create build router: %w", err)
		}
	} else {
		fmt.Println("Running in development mode")
		baseDir = "server"
		templateDir = "public"
		findAndReadFile = fnrDev
	}

	if _, err := loadTemplate("index.html"); err != nil {
		return nil, fmt.Errorf("failed to load main template: %v", err)
	}

	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("/", webHandler)
	mux.HandleFunc("/api/", jsonHandler)

	if config.IsSPAMode {
		mux.HandleFunc(spaPrefix, jsonHandler)
	}

	if config.EnableMetrics {
		if err := modules.NewMetrics(); err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %v", err)
		}
		fmt.Println("Metrics collection enabled, serving metrics at /metrics/server & /metrics/requests")
		return modules.MetricsMiddleware(mux), nil
	}

	return mux, nil
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
		Content:   template.HTML(content),
		Style:     template.CSS(style),
		Script:    template.JS(script),
		IsSPAMode: *isSPAMode,
	}, nil
}

func fnrDev(path, filename string) ([]byte, error) {
	fullPath := filepath.Join(baseDir, path, filename)
	return os.ReadFile(fullPath)
}

func fnrProd(path, filename string) ([]byte, error) {
	cacheKey := filepath.Join(path, filename)
	return reqCoalescer.Do(cacheKey, func() ([]byte, error) {
		// Try to get from cache first
		if value, ok := advancedCache.Get(cacheKey); ok {
			return value, nil
		}

		// If not in cache, try to get from build router
		filePath, ok := buildRouter.Route(cacheKey)
		if !ok {
			return nil, fmt.Errorf("file not found: %s", cacheKey)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
		}

		// Cache the content we just read
		advancedCache.Set(cacheKey, content, time.Now().Add(24*time.Hour))

		return content, nil
	})
}
