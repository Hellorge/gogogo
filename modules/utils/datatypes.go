package utils

import "time"

type FileInfo struct {
	ModTime   time.Time `json:"ModTime"`
	DistPath  string    `json:"DistPath"`
	DependsOn []string  `json:"DependsOn"`
}

type RadixNode struct {
	Path     string
	Children []*RadixNode
	// ChildMap map[string]*RadixNode
	FileInfo *FileInfo
	// Segments []string
}

func (n *RadixNode) Get(path string) *RadixNode {
	for _, child := range n.Children {
		if child.Path == path {
			return child
		}
	}
	return nil
}
