package templates

import (
	"fmt"
	"html/template"
	"path/filepath"
	"sync"

	"gogogo/modules/filemanager"
)

type TemplateEngine struct {
	templates     sync.Map
	templateMutex sync.RWMutex
	fm            *filemanager.FileManager
	dir           string
	GetTemplate   func(string) (*template.Template, error)
}

func New(fm *filemanager.FileManager, dir string, productionMode bool) *TemplateEngine {
	t := &TemplateEngine{
		fm:  fm,
		dir: dir,
	}

	if productionMode {
		t.GetTemplate = t.getProduction
	} else {
		t.GetTemplate = t.getDevelopment
	}

	return t
}

func (t *TemplateEngine) getProduction(name string) (*template.Template, error) {
	// Fast path - check sync.Map directly (it's already concurrent-safe)
	if tmpl, ok := t.templates.Load(name); ok {
		return tmpl.(*template.Template), nil
	}

	// Slow path - load and parse template
	path := filepath.Join(t.dir, name, "index.html")
	content, err := t.fm.GetContent(path)
	if err != nil {
		return nil, fmt.Errorf("error reading template file: %w", err)
	}

	tmpl, err := template.New(filepath.Base(name)).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %w", err)
	}

	// Store in sync.Map (handles race conditions internally)
	actual, loaded := t.templates.LoadOrStore(name, tmpl)
	if loaded {
		return actual.(*template.Template), nil
	}
	return tmpl, nil
}

func (t *TemplateEngine) getDevelopment(name string) (*template.Template, error) {
	path := filepath.Join(t.dir, name, "index.html")
	content, err := t.fm.GetContent(path)
	if err != nil {
		return nil, fmt.Errorf("error reading template file: %w", err)
	}

	return template.New(filepath.Base(name)).Parse(string(content))
}
