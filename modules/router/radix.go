package router

import (
	"time"
)

type FileInfo struct {
	ModTime   time.Time
	DistPath  string
	DependsOn []string
}

type RadixNode struct {
	Path     string
	Children []*RadixNode
	FileInfo *FileInfo
}

func (n *RadixNode) findChild(segment string) *RadixNode {
	for _, child := range n.Children {
		if child.Path == segment {
			return child
		}
	}
	return nil
}

func (n *RadixNode) Insert(segments []string, fileInfo *FileInfo) {
	if len(segments) == 0 {
		n.FileInfo = fileInfo
		return
	}

	segment := segments[0]
	child := n.findChild(segment)
	if child == nil {
		child = &RadixNode{
			Path:     segment,
			Children: make([]*RadixNode, 0, 4), // Pre-allocate for common case
		}
		n.Children = append(n.Children, child)
	}

	child.Insert(segments[1:], fileInfo)
}
