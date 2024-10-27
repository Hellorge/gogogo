package router

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gogogo/modules/cache"
	"gogogo/modules/coalescer"
	"gogogo/modules/config"
	"gogogo/modules/metaparser"
	"gogogo/modules/utils"
)

type Router struct {
	*http.ServeMux
	root            *RadixNode
	rwMutex         sync.RWMutex
	cfg             *config.Config
	cache           *cache.Cache
	coalescer       *coalescer.Coalescer
	templates       map[string]*template.Template // Changed from sync.Map for better performance
	templateMutex   sync.RWMutex
	findAndReadFile func(string, string, string) ([]byte, error)
}

type PageContent struct {
	Content   template.HTML
	Style     template.CSS
	StyleURL  string
	Script    template.JS
	ScriptURL string
	Meta      metaparser.MetaData
	IsSPAMode bool
}

func NewRouter(cfg *config.Config) (*Router, error) {
	r := &Router{
		ServeMux:  http.NewServeMux(),
		cfg:       cfg,
		templates: make(map[string]*template.Template),
	}

	if cfg.Server.ProductionMode {
		r.findAndReadFile = r.fnrProd
		if err := r.loadRadixTree(); err != nil {
			return nil, err
		}
	} else {
		r.findAndReadFile = r.fnrDev
	}

	// Pre-compile common templates at startup
	if err := r.preloadTemplates(); err != nil {
		return nil, err
	}

	r.setupHandlers()
	return r, nil
}

func (r *Router) preloadTemplates() error {
	r.templateMutex.Lock()
	defer r.templateMutex.Unlock()

	mainTemplate := filepath.Join(r.cfg.Templates.Main, "index.html")
	content, err := r.findAndReadFile("", r.cfg.Templates.Main, "index.html")
	if err != nil {
		return fmt.Errorf("error reading main template: %w", err)
	}

	tmpl, err := template.New(mainTemplate).Parse(string(content))
	if err != nil {
		return fmt.Errorf("error parsing main template: %w", err)
	}

	r.templates[mainTemplate] = tmpl
	return nil
}

func (r *Router) setupHandlers() {
	r.HandleFunc("/", r.webHandler)
	r.HandleFunc("/api/", r.apiHandler)
	r.HandleFunc("/static/", r.serveFile)
	r.HandleFunc("/core/", r.serveFile)
	if r.cfg.Server.SPAMode {
		r.HandleFunc(r.cfg.URLPrefixes.SPA, r.apiHandler)
	}
}

func (r *Router) fnrProd(BaseDir, path, filename string) ([]byte, error) {
	cacheKey := filepath.Join(BaseDir, path, filename)

	return r.coalescer.Do(cacheKey, func() ([]byte, error) {
		if r.cache != nil {
			if value, ok := r.cache.Get(cacheKey); ok {
				return value, nil
			}
		}

		filePath, ok := r.Route(cacheKey)
		if !ok {
			return nil, fmt.Errorf("file not found: %s", cacheKey)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
		}

		if r.cache != nil {
			r.cache.Set(cacheKey, content, time.Now().Add(r.cfg.Cache.DefaultExpiration))
		}

		return content, nil
	})
}

func (r *Router) fnrDev(BaseDir, path, filename string) ([]byte, error) {
	fullPath := filepath.Join(r.cfg.Directories.Web, BaseDir, path, filename)
	return os.ReadFile(fullPath)
}

func (r *Router) Route(fullPath string) (string, bool) {
	r.rwMutex.RLock()
	defer r.rwMutex.RUnlock()

	node := r.root
	if node == nil {
		return "", false
	}

	if fullPath == "/" {
		if node.FileInfo != nil {
			return node.FileInfo.DistPath, true
		}
		return "", false
	}

	path := fullPath
	if path[0] == '/' {
		path = path[1:]
	}

	var start, end int
	for end <= len(path) {
		if end == len(path) || path[end] == '/' {
			if end > start {
				segment := path[start:end]
				node = node.findChild(segment)
				if node == nil {
					return "", false
				}
			}
			start = end + 1
		}
		end++
	}

	if node.FileInfo != nil {
		return node.FileInfo.DistPath, true
	}
	return "", false
}

func (r *Router) loadContent(path string) (PageContent, error) {
	const (
		contentFile = "content.html"
		metaFile    = "meta.toml"
		styleFile   = "style.css"
		scriptFile  = "script.js"
	)

	content := PageContent{
		IsSPAMode: r.cfg.Server.SPAMode,
	}

	// Load main content - required
	htmlContent, err := r.findAndReadFile(r.cfg.Directories.Content, path, contentFile)
	if err != nil {
		return content, err
	}
	content.Content = template.HTML(htmlContent)

	// Load meta if available - optional
	var meta metaparser.MetaData
	if metaData, err := r.findAndReadFile(r.cfg.Directories.Content, path, metaFile); err == nil {
		if parsedMeta, err := metaparser.ParseMetaData(metaData); err == nil {
			meta = parsedMeta
			content.Meta = meta
		}
	}

	// Handle style - always try loading based on meta or fallback
	if meta.InlineStyle {
		if style, err := r.findAndReadFile(r.cfg.Directories.Content, path, styleFile); err == nil {
			content.Style = template.CSS(style)
		}
	} else {
		content.StyleURL = "/" + path + "/style.css"
	}

	// Handle script - always try loading based on meta or fallback
	if meta.InlineScript {
		if script, err := r.findAndReadFile(r.cfg.Directories.Content, path, scriptFile); err == nil {
			content.Script = template.JS(script)
		}
	} else {
		content.ScriptURL = "/" + path + "/script.js"
	}

	return content, nil
}

func (r *Router) getTemplate(key string) (*template.Template, bool) {
	r.templateMutex.RLock()
	tmpl, ok := r.templates[key]
	r.templateMutex.RUnlock()
	return tmpl, ok
}

func (r *Router) webHandler(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "/" {
		path = "home"
	} else {
		path = path[1:] // Faster than strings.TrimPrefix
	}

	content, err := r.loadContent(path)
	if err != nil {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}

	tmpl, ok := r.getTemplate(filepath.Join(r.cfg.Templates.Main, "index.html"))
	if !ok {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, content); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (r *Router) apiHandler(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path[5:] // Skip "/api/"
	if r.cfg.Server.SPAMode {
		prefixLen := len(r.cfg.URLPrefixes.SPA)
		if len(path) > prefixLen {
			path = path[prefixLen:]
		}
	}

	if path == "" {
		path = "home"
	}

	content, err := r.loadContent(path)
	if err != nil {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(content)
}

func (r *Router) serveFile(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "/" {
		path = "home"
	} else {
		path = path[1:]
	}

	content, err := r.findAndReadFile("", "", path)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", utils.GetMimeType(filepath.Ext(path)))
	http.ServeContent(w, req, path, time.Now(), strings.NewReader(string(content)))
}

func (r *Router) loadRadixTree() error {
	file, err := os.Open(filepath.Join(r.cfg.Directories.Meta, "router_binary.bin"))
	if err != nil {
		return fmt.Errorf("failed to open build radix tree: %w", err)
	}
	defer file.Close()

	dec := gob.NewDecoder(file)
	root := &RadixNode{
		Children: make([]*RadixNode, 0, 8), // Pre-allocate for common case
	}

	if err := dec.Decode(root); err != nil {
		return fmt.Errorf("failed to decode radix tree: %w", err)
	}

	r.root = root
	return nil
}
