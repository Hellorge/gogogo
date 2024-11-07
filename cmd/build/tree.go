package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/fatih/color"
)

type TreeNode struct {
	name     string
	alias    string
	isDir    bool
	children []*TreeNode
}

func NewTreeCommand() *TreeCommand {
	return &TreeCommand{
		aliasMap: make(map[string]string),
	}
}

type TreeCommand struct {
	aliasMap map[string]string
}

func (t *TreeCommand) Execute(rootPath string) error {
	// First pass: collect all aliases
	if err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			metaPath := filepath.Join(path, "meta.toml")
			if _, err := os.Stat(metaPath); err == nil {
				data, err := os.ReadFile(metaPath)
				if err != nil {
					return err
				}

				meta := &MetaData{}
				if err := toml.Unmarshal(data, meta); err != nil {
					return err
				}

				if meta.Alias != "" {
					relPath, _ := filepath.Rel(rootPath, path)
					t.aliasMap[relPath] = meta.Alias
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// Build tree structure
	root := &TreeNode{
		name:  filepath.Base(rootPath),
		isDir: true,
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == rootPath {
			return nil
		}

		relPath, _ := filepath.Rel(rootPath, path)
		parts := strings.Split(relPath, string(os.PathSeparator))

		currentNode := root
		currentPath := ""

		for i, part := range parts {
			currentPath = filepath.Join(currentPath, part)
			found := false

			for _, child := range currentNode.children {
				if child.name == part {
					currentNode = child
					found = true
					break
				}
			}

			if !found {
				newNode := &TreeNode{
					name:  part,
					isDir: info.IsDir() || i < len(parts)-1,
				}

				if info.IsDir() {
					if alias, exists := t.aliasMap[currentPath]; exists {
						newNode.alias = alias
					}
				}

				currentNode.children = append(currentNode.children, newNode)
				currentNode = newNode
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Sort children at each level
	t.sortTree(root)

	// Print the tree
	fmt.Println()
	t.printTree(root, "", true)
	fmt.Println()

	return nil
}

func (t *TreeCommand) sortTree(node *TreeNode) {
	sort.Slice(node.children, func(i, j int) bool {
		// Directories come first
		if node.children[i].isDir != node.children[j].isDir {
			return node.children[i].isDir
		}
		return node.children[i].name < node.children[j].name
	})

	for _, child := range node.children {
		t.sortTree(child)
	}
}

func (t *TreeCommand) printTree(node *TreeNode, prefix string, isLast bool) {
	// Colors for different elements
	dirColor := color.New(color.FgBlue, color.Bold)
	fileColor := color.New(color.FgWhite)
	aliasColor := color.New(color.FgGreen)

	// Connection characters
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	if node.name != "" {
		fmt.Print(prefix)
		fmt.Print(connector)

		if node.isDir {
			dirColor.Print(node.name)
		} else {
			fileColor.Print(node.name)
		}

		if node.alias != "" {
			fmt.Print(" ")
			aliasColor.Printf("-> /%s", node.alias)
		}
		fmt.Println()
	}

	newPrefix := prefix
	if node.name != "" {
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
	}

	for i, child := range node.children {
		isLastChild := i == len(node.children)-1
		t.printTree(child, newPrefix, isLastChild)
	}
}
