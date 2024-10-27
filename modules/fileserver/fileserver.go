package fileserver

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"gogogo/modules/cache"
	"gogogo/modules/coalescer"
	"gogogo/modules/config"
	"gogogo/modules/utils"
)

type FileServer struct {
	cache     *cache.Cache
	coalescer *coalescer.Coalescer
	config    *config.Config
	readFile  func(string, string, string) ([]byte, error)
}

func NewFileServer(cfg *config.Config, cache *cache.Cache, coalescer *coalescer.Coalescer, readFile func(string, string, string) ([]byte, error)) *FileServer {
	return &FileServer{
		cache:     cache,
		coalescer: coalescer,
		config:    cfg,
		readFile:  readFile,
	}
}

func (fs *FileServer) ServeFile(w http.ResponseWriter, r *http.Request, path string) error {
	content, err := fs.readWithCache(path)
	if err != nil {
		return err
	}

	ext := filepath.Ext(path)
	w.Header().Set("Content-Type", utils.GetMimeType(ext))
	http.ServeContent(w, r, path, time.Now(), strings.NewReader(string(content)))
	return nil
}

func (fs *FileServer) readWithCache(path string) ([]byte, error) {
	if fs.coalescer == nil {
		return fs.readFile("", "", path)
	}

	return fs.coalescer.Do(path, func() ([]byte, error) {
		if fs.cache != nil {
			if value, ok := fs.cache.Get(path); ok {
				return value, nil
			}
		}

		content, err := fs.readFile("", "", path)
		if err != nil {
			return nil, err
		}

		if fs.cache != nil {
			fs.cache.Set(path, content, time.Now().Add(fs.config.Cache.DefaultExpiration))
		}

		return content, nil
	})
}
