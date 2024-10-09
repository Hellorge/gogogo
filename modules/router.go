package modules

import (
	"encoding/gob"
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

type BinaryTrieNode struct {
    Children map[string]*BinaryTrieNode
    FileInfo *FileInfo
}

type Router struct {
    root *BinaryTrieNode
}

func init() {
    // Register the FileInfo type with gob
    gob.Register(FileInfo{})
}

func NewRouter(buildTriePath string) (*Router, error) {
    file, err := os.Open(buildTriePath)
    if err != nil {
        return nil, fmt.Errorf("failed to open build trie: %w", err)
    }
    defer file.Close()

    var root BinaryTrieNode
    dec := gob.NewDecoder(file)
    if err := dec.Decode(&root); err != nil {
        return nil, fmt.Errorf("failed to decode build trie: %w", err)
    }

    return &Router{root: &root}, nil
}

func (r *Router) Route(path string) (string, bool) {
    node := r.root
    parts := strings.Split(strings.Trim(path, "/"), "/")
    for _, part := range parts {
        if child, exists := node.Children[part]; exists {
            node = child
        } else {
            return "", false
        }
    }
    if node.FileInfo != nil {
        return node.FileInfo.DistPath, true
    }
    return "", false
}
