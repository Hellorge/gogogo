package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"sync"

	"gogogo/modules/filemanager"
	"gogogo/modules/metaparser"
)

// Pre-computed paths
const (
	contentFile = "content.html"
	metaFile    = "meta.toml"
	styleFile   = "style.css"
	scriptFile  = "script.js"
)

// TemplateData with zero allocation defaults
var defaultMeta = &metaparser.MetaData{}

type TemplateData struct {
	Content   template.HTML
	Style     template.CSS
	Script    template.JS
	StyleURL  string
	ScriptURL string
	Meta      *metaparser.MetaData
	IsSPAMode bool
}

// Handlers using shared buffer pools
type WebHandler struct {
	fm          *filemanager.FileManager
	template    *template.Template
	contentPath string
	bufferPool  sync.Pool
}

type SPAHandler struct {
	fm          *filemanager.FileManager
	contentPath string
	bufferPool  sync.Pool
}

type StaticHandler struct {
	fm *filemanager.FileManager
}

type APIHandler struct {
	fm          *filemanager.FileManager
	contentPath string
	bufferPool  sync.Pool
}

// ContentLoader for efficient content loading
type ContentLoader struct {
	content []byte
	meta    []byte
	style   []byte
	script  []byte
	err     error
	wg      sync.WaitGroup
}

func NewWebHandler(fm *filemanager.FileManager, tmpl *template.Template, contentPath string) *WebHandler {
	return &WebHandler{
		fm:          fm,
		template:    tmpl,
		contentPath: contentPath,
		bufferPool: sync.Pool{
			New: func() interface{} {
				return &TemplateData{Meta: defaultMeta}
			},
		},
	}
}

func NewSPAHandler(fm *filemanager.FileManager, contentPath string) *SPAHandler {
	return &SPAHandler{
		fm:          fm,
		contentPath: contentPath,
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 4096)
			},
		},
	}
}

func NewStaticHandler(fm *filemanager.FileManager) *StaticHandler {
	return &StaticHandler{fm: fm}
}

func NewAPIHandler(fm *filemanager.FileManager, contentPath string) *APIHandler {
	return &APIHandler{
		fm:          fm,
		contentPath: contentPath,
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 1024)
			},
		},
	}
}

// loadContent loads all content in parallel
func loadContent(fm *filemanager.FileManager, basePath string) *ContentLoader {
	cl := &ContentLoader{}
	cl.wg.Add(4)

	// Load content and meta in parallel
	go func() {
		defer cl.wg.Done()
		cl.content, cl.err = fm.GetContent(filepath.Join(basePath, contentFile))
	}()

	go func() {
		defer cl.wg.Done()
		cl.meta, _ = fm.GetContent(filepath.Join(basePath, metaFile))
	}()

	go func() {
		defer cl.wg.Done()
		cl.style, _ = fm.GetContent(filepath.Join(basePath, styleFile))
	}()

	go func() {
		defer cl.wg.Done()
		cl.script, _ = fm.GetContent(filepath.Join(basePath, scriptFile))
	}()

	cl.wg.Wait()
	return cl
}

func (h *WebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" || path == "" {
		path = "home"
	}

	contentPath := filepath.Join(h.contentPath, path)
	cl := loadContent(h.fm, contentPath)
	if cl.err != nil {
		http.NotFound(w, r)
		return
	}

	// Get data from pool
	data := h.bufferPool.Get().(*TemplateData)
	defer h.bufferPool.Put(data)

	// Reset data
	*data = TemplateData{
		Content:   template.HTML(cl.content),
		Meta:      defaultMeta,
		IsSPAMode: true,
	}

	if meta, err := metaparser.ParseMetaData(cl.meta); err == nil {
		data.Meta = meta
	}

	if data.Meta.InlineStyle && len(cl.style) > 0 {
		data.Style = template.CSS(cl.style)
	} else {
		data.StyleURL = filepath.Join(path, styleFile)
	}

	if data.Meta.InlineScript && len(cl.script) > 0 {
		data.Script = template.JS(cl.script)
	} else {
		data.ScriptURL = filepath.Join(path, scriptFile)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.template.Execute(w, data)
}

func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" || path == "" {
		path = "home"
	}

	contentPath := filepath.Join(h.contentPath, path)
	fmt.Println(contentPath)
	cl := loadContent(h.fm, contentPath)
	if cl.err != nil {
		http.NotFound(w, r)
		return
	}

	// Get buffer from pool
	buf := h.bufferPool.Get().([]byte)
	defer h.bufferPool.Put(buf)

	// Reuse buffer for response
	resp := struct {
		Content   string               `json:"Content"`
		Style     string               `json:"Style,omitempty"`
		Script    string               `json:"Script,omitempty"`
		StyleURL  string               `json:"StyleUrl,omitempty"`
		ScriptURL string               `json:"ScriptUrl,omitempty"`
		Meta      *metaparser.MetaData `json:"Meta,omitempty"`
		IsSPAMode bool
	}{
		Content: string(cl.content),
	}

	resp.IsSPAMode = true
	if meta, err := metaparser.ParseMetaData(cl.meta); err == nil {
		resp.Meta = meta
	}

	if resp.Meta.InlineStyle && len(cl.style) > 0 {
		resp.Style = string(cl.style)
	} else {
		resp.StyleURL = filepath.Join(path, styleFile)
	}

	if resp.Meta.InlineScript && len(cl.script) > 0 {
		resp.Script = string(cl.script)
	} else {
		resp.ScriptURL = filepath.Join(path, scriptFile)
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.Encode(resp)
}

func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	file, err := h.fm.OpenFile(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, r.URL.Path, info.ModTime(), file)
}

func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[5:] // strip /api/
	if path == "" {
		path = "home"
	}

	content, err := h.fm.GetContent(filepath.Join(h.contentPath, path, contentFile))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Get buffer from pool
	buf := h.bufferPool.Get().([]byte)
	defer h.bufferPool.Put(buf)

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.Encode(map[string]string{"content": string(content)})
}
