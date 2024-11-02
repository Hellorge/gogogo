package filemanager

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"gogogo/modules/cache"
	"gogogo/modules/coalescer"
	"gogogo/modules/fileaccess"
	"gogogo/modules/router"
)

var (
	ErrNotFound = errors.New("file not found")
)

type FileManager struct {
	fileAccess *fileaccess.FileAccess
	cache      *cache.Cache
	coalescer  *coalescer.Coalescer
	router     *router.Router
	rootDir    string
	GetContent func(path string) ([]byte, error)
	OpenFile   func(path string) (*os.File, error)
	Exists     func(path string) bool
}

type Config struct {
	RootDir string
	Router  *router.Router // Will be nil in dev mode
}

func New(fa *fileaccess.FileAccess, ca *cache.Cache, co *coalescer.Coalescer, cfg Config) *FileManager {
	fm := &FileManager{
		fileAccess: fa,
		cache:      ca,
		coalescer:  co,
		router:     cfg.Router,
		rootDir:    cfg.RootDir,
	}

	// Set the appropriate GetContent function based on whether Router exists
	if cfg.Router != nil {
		fm.GetContent = fm.getProduction
		fm.Exists = fm.ExistsProduction
		fm.OpenFile = fm.OpenProduction
	} else {
		fm.GetContent = fm.getDevelopment
		fm.Exists = fm.ExistsDevelopment
		fm.OpenFile = fm.OpenDevelopment
	}

	return fm
}

func (fm *FileManager) ExistsDevelopment(path string) bool {
	_, err := fm.fileAccess.Stat(filepath.Join(fm.rootDir, path))
	return err == nil
}

func (fm *FileManager) ExistsProduction(path string) bool {
	_, ok := fm.router.Route(path)
	return ok
}

func (fm *FileManager) getDevelopment(path string) ([]byte, error) {
	return fm.fileAccess.Read(filepath.Join(fm.rootDir, path))
}

func (fm *FileManager) getProduction(path string) ([]byte, error) {
	distPath, ok := fm.router.Route(path)
	if !ok {
		return nil, ErrNotFound
	}

	return fm.coalescer.Do(distPath, func() ([]byte, error) {
		if fm.cache != nil {
			if data, ok := fm.cache.Get(distPath); ok {
				return data, nil
			}
		}

		data, err := fm.fileAccess.Read(distPath)
		if err != nil {
			return nil, err
		}

		if fm.cache != nil {
			fm.cache.Set(distPath, data, time.Now().Add(24*time.Hour))
		}

		return data, nil
	})
}

// OpenFile opens a file for direct reading (used by ServeContent)
func (fm *FileManager) OpenDevelopment(path string) (*os.File, error) {
	return fm.fileAccess.Open(path)
}

func (fm *FileManager) OpenProduction(path string) (*os.File, error) {
	distPath, ok := fm.router.Route(path)
	if !ok {
		return nil, ErrNotFound
	}
	return fm.fileAccess.Open(distPath)
}
