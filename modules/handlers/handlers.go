package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"
	"sync"

	"gogogo/modules/filemanager"
	"gogogo/modules/metaparser"
	"gogogo/modules/pathbuilder"
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

type WebHandler struct {
	fm          *filemanager.FileManager
	template    *template.Template
	contentPath string
	SPAMode     bool
}

type SPAHandler struct {
	fm          *filemanager.FileManager
	contentPath string
	SPAMode     bool
}

type StaticHandler struct {
	fm *filemanager.FileManager
}

type APIHandler struct {
	fm          *filemanager.FileManager
	contentPath string
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

func (cl *ContentLoader) reset() {
	cl.content = nil
	cl.meta = nil
	cl.style = nil
	cl.script = nil
	cl.err = nil
}

func NewWebHandler(fm *filemanager.FileManager, tmpl *template.Template, contentPath string, SPAMode bool) *WebHandler {
	return &WebHandler{
		fm:          fm,
		template:    tmpl,
		contentPath: contentPath,
		SPAMode:     SPAMode,
	}
}

func NewSPAHandler(fm *filemanager.FileManager, contentPath string, SPAMode bool) *SPAHandler {
	return &SPAHandler{
		fm:          fm,
		contentPath: contentPath,
		SPAMode:     SPAMode,
	}
}

func NewStaticHandler(fm *filemanager.FileManager) *StaticHandler {
	return &StaticHandler{fm: fm}
}

func NewAPIHandler(fm *filemanager.FileManager, contentPath string) *APIHandler {
	return &APIHandler{
		fm:          fm,
		contentPath: contentPath,
	}
}

func loadContent(fm *filemanager.FileManager, dir string, path string) *ContentLoader {
	cl := &ContentLoader{}
	cl.reset()

	pb := pathbuilder.AcquirePathBuilder()
	defer pb.Release()

	contentPath := pb.Join(dir, path, contentFile).String()
	metaPath := pb.Join(dir, path, metaFile).String()
	stylePath := pb.Join(dir, path, styleFile).String()
	scriptPath := pb.Join(dir, path, scriptFile).String()

	cl.wg.Add(4)

	go func() {
		defer cl.wg.Done()
		cl.content, cl.err = fm.GetContent(contentPath)
	}()

	go func() {
		defer cl.wg.Done()
		cl.meta, _ = fm.GetContent(metaPath)
	}()

	go func() {
		defer cl.wg.Done()
		cl.style, _ = fm.GetContent(stylePath)
	}()

	go func() {
		defer cl.wg.Done()
		cl.script, _ = fm.GetContent(scriptPath)
	}()

	cl.wg.Wait()
	return cl
}

func (h *WebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if len(path) <= 1 {
		path = "home"
	}

	cl := loadContent(h.fm, h.contentPath, path)

	if cl.err != nil {
		http.NotFound(w, r)
		return
	}

	// Reset data
	data := TemplateData{
		Content:   template.HTML(cl.content),
		Meta:      defaultMeta,
		IsSPAMode: h.SPAMode,
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
	if len(path) <= 1 {
		path = "home"
	}

	cl := loadContent(h.fm, h.contentPath, path)
	if cl.err != nil {
		http.NotFound(w, r)
		return
	}

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
		Content:   string(cl.content),
		IsSPAMode: h.SPAMode,
		Meta:      defaultMeta,
	}

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
	if len(path) <= 1 {
		path = "home"
	}

	content, err := h.fm.GetContent(filepath.Join(h.contentPath, path, contentFile))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.Encode(map[string]string{"content": string(content)})
}
