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
}

func New(fm *filemanager.FileManager, dir string) *TemplateEngine {
	return &TemplateEngine{
		fm:  fm,
		dir: dir,
	}
}

func (t *TemplateEngine) GetTemplate(name string) (*template.Template, error) {
	// Check cache first
	t.templateMutex.RLock()
	if tmpl, ok := t.templates.Load(name); ok {
		t.templateMutex.RUnlock()
		return tmpl.(*template.Template), nil
	}
	t.templateMutex.RUnlock()

	// Not in cache, load and parse
	t.templateMutex.Lock()
	defer t.templateMutex.Unlock()

	// Double-check after acquiring lock
	if tmpl, ok := t.templates.Load(name); ok {
		return tmpl.(*template.Template), nil
	}

	path := filepath.Join(t.dir, name, "index.html")
	content, err := t.fm.GetContent(path)
	if err != nil {
		return nil, fmt.Errorf("error reading template file: %w", err)
	}

	tmpl, err := template.New(filepath.Base(name)).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %w", err)
	}

	t.templates.Store(name, tmpl)
	return tmpl, nil
}
