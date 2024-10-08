package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type FileInfo struct {
	ModTime   time.Time `json:"ModTime"`
	DistPath  string    `json:"DistPath"`
	DependsOn []string  `json:"DependsOn"`
}

type TrieNode struct {
	children map[string]*TrieNode
	fileInfo *FileInfo
}

type Router struct {
	root *TrieNode
}

func NewRouter(buildFileInfoPath string) (*Router, error) {
	data, err := os.ReadFile(buildFileInfoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read build file info: %w", err)
	}

	var fileInfoMap map[string]FileInfo
	if err := json.Unmarshal(data, &fileInfoMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal build file info: %w", err)
	}

	router := &Router{
		root: &TrieNode{children: make(map[string]*TrieNode)},
	}

	for urlPath, info := range fileInfoMap {
		router.insert(urlPath, &info)
	}

	return router, nil
}

func (r *Router) insert(urlPath string, info *FileInfo) {
	node := r.root
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	for _, part := range parts {
		if _, exists := node.children[part]; !exists {
			node.children[part] = &TrieNode{children: make(map[string]*TrieNode)}
		}
		node = node.children[part]
	}
	node.fileInfo = info
}

func (r *Router) Route(urlPath string) (string, bool) {
	node := r.root
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	for _, part := range parts {
		if child, exists := node.children[part]; exists {
			node = child
		} else {
			return "", false
		}
	}
	if node.fileInfo != nil {
		return node.fileInfo.DistPath, true
	}
	return "", false
}
