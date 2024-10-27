package templates

import (
	"fmt"
	"html/template"
	"path/filepath"
	"sync"

	"gogogo/modules/config"
	"gogogo/modules/metaparser"
)

type TemplateEngine struct {
	templates     sync.Map
	templateMutex sync.RWMutex
	config        *config.Config
	readFile      func(string, string, string) ([]byte, error)
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

func NewTemplateEngine(cfg *config.Config, readFileFn func(string, string, string) ([]byte, error)) *TemplateEngine {
	return &TemplateEngine{
		config:   cfg,
		readFile: readFileFn,
	}
}

func (t *TemplateEngine) GetTemplate(dir, name string) (*template.Template, error) {
	tmplKey := filepath.Join(dir, name)

	// Try from cache first
	t.templateMutex.RLock()
	if tmpl, ok := t.templates.Load(tmplKey); ok {
		t.templateMutex.RUnlock()
		return tmpl.(*template.Template), nil
	}
	t.templateMutex.RUnlock()

	// Not in cache, load and parse
	t.templateMutex.Lock()
	defer t.templateMutex.Unlock()

	// Double-check after acquiring lock
	if tmpl, ok := t.templates.Load(tmplKey); ok {
		return tmpl.(*template.Template), nil
	}

	content, err := t.readFile("", dir, name)
	if err != nil {
		return nil, fmt.Errorf("error reading template file: %w", err)
	}

	tmpl, err := template.New(tmplKey).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %w", err)
	}

	t.templates.Store(tmplKey, tmpl)
	return tmpl, nil
}

func (t *TemplateEngine) LoadContent(path string) (PageContent, error) {
	content := PageContent{
		IsSPAMode: t.config.Server.SPAMode,
	}

	htmlContent, err := t.readFile(t.config.Directories.Content, path, "content.html")
	if err != nil {
		return content, fmt.Errorf("content file error: %w", err)
	}

	metaData, _ := t.readFile(t.config.Directories.Content, path, "meta.toml")
	meta, _ := metaparser.ParseMetaData(metaData)

	content.Content = template.HTML(htmlContent)
	content.Meta = meta

	if meta.InlineStyle {
		style, _ := t.readFile(t.config.Directories.Content, path, "style.css")
		content.Style = template.CSS(style)
	} else {
		content.StyleURL = "/" + path + "/style.css"
	}

	if meta.InlineScript {
		script, _ := t.readFile(t.config.Directories.Content, path, "script.js")
		content.Script = template.JS(script)
	} else {
		content.ScriptURL = "/" + path + "/script.js"
	}

	return content, nil
}
