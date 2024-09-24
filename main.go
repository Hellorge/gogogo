package main

import (
	"container/list"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"webSPA/metrics"
)

type PageContent struct {
	Content   template.HTML
	Style     template.CSS
	Script    template.JS
	IsSPAMode bool
}

type FileInfo struct {
	DistPath string `json:"DistPath"`
}

type CacheEntry struct {
	Content     PageContent
	Expiration  time.Time
	listElement *list.Element
}

type DynamicCache struct {
	entries       sync.Map
	maxEntries    int64
	evictionList  *list.List
	mutex         sync.Mutex
	sizeThreshold int64
}

type CacheCoalescer struct {
	cache    *DynamicCache
	calls    map[string]*Call
	mutex    sync.RWMutex
	cacheTTL time.Duration
}

type Call struct {
	Done    chan struct{}
	Content PageContent
	Err     error
}

var (
	templateDir   string
	templates     = make(map[string]*template.Template)
	templateMutex sync.RWMutex

	isProduction = flag.Bool("prod", false, "Run in production mode")
	IsSPAMode    = flag.Bool("spa", false, "Run in SPA mode")

	enableMetrics = flag.Bool("metrics", false, "Enable metrics collection")

	baseDir     string
	staticDir   = "public"
	fileInfoMap map[string]string

	cacheCoalescer  *CacheCoalescer
	findAndReadFile func(string, string) ([]byte, error)
)

const spaPrefix = "/__spa__/"

func main() {
	flag.Parse()

	if *isProduction {
		fmt.Println("Running in production mode")
		baseDir = "dist"
		templateDir = "public"
		loadFileInfo()
		findAndReadFile = fnrProd
	} else {
		fmt.Println("Running in development mode")
		baseDir = "server"
		templateDir = "public"
		findAndReadFile = fnrDev
	}

	wrapHandler := func(h http.HandlerFunc) http.HandlerFunc { return h }

	if *enableMetrics {
		metrics.Init()
		wrapHandler = metrics.Middleware
		fmt.Println("Metrics collection enabled, serving metrics at /metrics/server & metrics/requests")
	}

	cacheCoalescer = NewCacheCoalescer(5 * time.Minute)

	if _, err := loadTemplate("index.html"); err != nil {
		log.Fatalf("Failed to load main template: %v", err)
	}

	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", wrapHandler(webHandler))
	http.HandleFunc("/api/", wrapHandler(jsonHandler))
	http.HandleFunc(spaPrefix, wrapHandler(jsonHandler))

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
	return cacheCoalescer.Get(path, func() (PageContent, error) {
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
			IsSPAMode: *IsSPAMode,
		}, nil
	})
}

func fnrDev(path, filename string) ([]byte, error) {
	fullPath := filepath.Join(baseDir, path, filename)
	return ioutil.ReadFile(fullPath)
}

func fnrProd(path, filename string) ([]byte, error) {
	cacheKey := filepath.Join("server", path, filename)

	distPath, ok := fileInfoMap[cacheKey]
	if !ok {
		return nil, fmt.Errorf("file not found in production map: %s", filename)
	}

	content, err := ioutil.ReadFile(distPath)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", distPath, err)
	}

	return content, nil
}

func loadFileInfo() {
	data, err := ioutil.ReadFile("build_file_info.json")
	if err != nil {
		log.Fatalf("Error reading build_file_info.json: %v", err)
	}

	var fullFileInfo map[string]FileInfo
	if err := json.Unmarshal(data, &fullFileInfo); err != nil {
		log.Fatalf("Error unmarshaling build_file_info.json: %v", err)
	}

	fileInfoMap = make(map[string]string, len(fullFileInfo))
	for key, info := range fullFileInfo {
		fileInfoMap[key] = info.DistPath
	}
}

// DynamicCache methods
func NewDynamicCache() *DynamicCache {
	dc := &DynamicCache{
		evictionList:  list.New(),
		maxEntries:    1000, // Default size, adjust as needed
		sizeThreshold: 900,  // 90% of maxEntries
	}
	go dc.periodicCleanup()
	return dc
}

func (dc *DynamicCache) Get(key string) (PageContent, bool) {
	if value, ok := dc.entries.Load(key); ok {
		entry := value.(*CacheEntry)
		if time.Now().Before(entry.Expiration) {
			dc.mutex.Lock()
			dc.evictionList.MoveToFront(entry.listElement)
			dc.mutex.Unlock()
			return entry.Content, true
		}
		dc.entries.Delete(key)
	}
	return PageContent{}, false
}

func (dc *DynamicCache) Set(key string, content PageContent, expiration time.Time) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	if entry, ok := dc.entries.Load(key); ok {
		dc.evictionList.Remove(entry.(*CacheEntry).listElement)
	} else if int64(dc.evictionList.Len()) >= atomic.LoadInt64(&dc.maxEntries) {
		oldest := dc.evictionList.Back()
		if oldest != nil {
			dc.evictionList.Remove(oldest)
			dc.entries.Delete(oldest.Value.(string))
		}
	}

	entry := &CacheEntry{
		Content:     content,
		Expiration:  expiration,
		listElement: dc.evictionList.PushFront(key),
	}
	dc.entries.Store(key, entry)

	if int64(dc.evictionList.Len()) > atomic.LoadInt64(&dc.sizeThreshold) {
		go dc.adjustCacheSize()
	}
}

func (dc *DynamicCache) adjustCacheSize() {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	availableMemory := m.Sys - m.HeapAlloc
	targetMemory := availableMemory / 4 // Use up to 25% of available memory

	estimatedEntrySize := int64(1024) // 1KB per entry, adjust based on actual size

	newMaxEntries := int64(targetMemory / uint64(estimatedEntrySize))

	if newMaxEntries < 100 {
		newMaxEntries = 100 // Minimum cache size
	} else if newMaxEntries > 1000000 {
		newMaxEntries = 1000000 // Maximum cache size
	}

	for dc.evictionList.Len() > int(newMaxEntries) {
		oldest := dc.evictionList.Back()
		if oldest != nil {
			dc.evictionList.Remove(oldest)
			dc.entries.Delete(oldest.Value.(string))
		}
	}

	atomic.StoreInt64(&dc.maxEntries, newMaxEntries)
	atomic.StoreInt64(&dc.sizeThreshold, newMaxEntries*9/10) // Set threshold to 90% of max size
}

func (dc *DynamicCache) periodicCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		dc.removeExpiredEntries()
	}
}

func (dc *DynamicCache) removeExpiredEntries() {
	now := time.Now()
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	dc.entries.Range(func(key, value interface{}) bool {
		entry := value.(*CacheEntry)
		if now.After(entry.Expiration) {
			dc.evictionList.Remove(entry.listElement)
			dc.entries.Delete(key)
		}
		return true
	})
}

// CacheCoalescer methods

func NewCacheCoalescer(cacheTTL time.Duration) *CacheCoalescer {
	return &CacheCoalescer{
		cache:    NewDynamicCache(),
		calls:    make(map[string]*Call),
		cacheTTL: cacheTTL,
	}
}

func (cc *CacheCoalescer) Get(path string, loader func() (PageContent, error)) (PageContent, error) {
	if content, ok := cc.cache.Get(path); ok {
		return content, nil
	}

	cc.mutex.Lock()
	if call, ok := cc.calls[path]; ok {
		cc.mutex.Unlock()
		<-call.Done
		return call.Content, call.Err
	}

	call := &Call{Done: make(chan struct{})}
	cc.calls[path] = call
	cc.mutex.Unlock()

	content, err := loader()

	cc.mutex.Lock()
	delete(cc.calls, path)
	cc.mutex.Unlock()

	if err == nil {
		cc.cache.Set(path, content, time.Now().Add(cc.cacheTTL))
	}

	call.Content = content
	call.Err = err
	close(call.Done)

	return content, err
}
