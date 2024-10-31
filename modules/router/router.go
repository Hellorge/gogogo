package router

import (
	"encoding/gob"
	"os"
	"sync"
)

type Router struct {
	root    *RadixNode
	rwMutex sync.RWMutex
}

type FileInfo struct {
	DistPath string
}

type RadixNode struct {
	Path     string
	Children []*RadixNode
	FileInfo *FileInfo
}

func New() *Router {
	return &Router{
		root: &RadixNode{
			Children: make([]*RadixNode, 0, 8),
		},
	}
}

func LoadFromBinary(binPath string) (*Router, error) {
	f, err := os.Open(binPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	root := &RadixNode{}
	dec := gob.NewDecoder(f)
	if err := dec.Decode(root); err != nil {
		return nil, err
	}

	return &Router{
		root: root,
	}, nil
}

// Route finds the dist path for a given request path
func (r *Router) Route(path string) (string, bool) {
	r.rwMutex.RLock()
	fileInfo := r.findRoute(path)
	r.rwMutex.RUnlock()

	if fileInfo == nil {
		return "", false
	}

	return fileInfo.DistPath, true
}

func (r *Router) findRoute(path string) *FileInfo {
	node := r.root
	if len(path) <= 1 {
		return node.FileInfo
	}

	var start, end int
	for end <= len(path) {
		if end == len(path) || path[end] == '/' {
			if end > start {
				segment := path[start:end]
				found := false
				for _, child := range node.Children {
					if child.Path == segment {
						node = child
						found = true
						break
					}
				}
				if !found {
					return nil
				}
			}
			start = end + 1
		}
		end++
	}

	return node.FileInfo
}
