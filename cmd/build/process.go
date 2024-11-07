package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"gogogo/modules/router"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

type ProcessResult struct {
	FileInfo     router.FileInfo
	Content      []byte
	Hash         string
	Dependencies []string
}

func (w *Worker) processFile(item WorkItem) (ProcessResult, error) {
	if w.ctx.dryRun {
		return ProcessResult{
			FileInfo: router.FileInfo{
				ModTime:  item.Info.ModTime(),
				DistPath: filepath.Join(w.ctx.config.Directories.Dist, item.AliasedPath),
			},
		}, nil
	}

	content, err := os.ReadFile(item.Path)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("error reading file: %w", err)
	}

	hash := md5.Sum(content)
	hashString := hex.EncodeToString(hash[:])

	if entry, ok := w.ctx.buildCache.Get(item.RelPath); ok && entry.Hash == hashString {
		return ProcessResult{
			FileInfo: router.FileInfo{
				ModTime:   item.Info.ModTime(),
				DistPath:  entry.DistPath,
				DependsOn: findDependencies(content),
			},
			Content:      entry.Content,
			Hash:         entry.Hash,
			Dependencies: findDependencies(content),
		}, nil
	}

	ext := filepath.Ext(item.Path)
	var mimeType string
	switch ext {
	case ".html":
		mimeType = "text/html"
	case ".css":
		mimeType = "text/css"
	case ".js":
		mimeType = "text/javascript"
	}

	var minified []byte
	if ext == ".js" {
		result := api.Transform(string(content), api.TransformOptions{
			Loader:            api.LoaderJS,
			MinifyWhitespace:  true,
			MinifyIdentifiers: true,
			MinifySyntax:      true,
			Sourcemap:         api.SourceMapInline,
		})
		if len(result.Errors) > 0 {
			return ProcessResult{}, fmt.Errorf("error minifying: %v", result.Errors)
		}
		minified = result.Code
	} else if mimeType != "" {
		var err error
		minified, err = w.ctx.minifier.Bytes(mimeType, content)
		if err != nil {
			return ProcessResult{}, fmt.Errorf("error minifying: %w", err)
		}
	} else {
		minified = content
	}

	minifiedHash := md5.Sum(minified)
	fileName := fmt.Sprintf("%s.%s%s",
		strings.TrimSuffix(filepath.Base(item.Path), ext),
		hex.EncodeToString(minifiedHash[:])[:8],
		ext,
	)

	relDir := filepath.Dir(item.RelPath)
	outPath := filepath.Join(w.ctx.outputDir, relDir, fileName)

	if err := atomicWrite(outPath, minified); err != nil {
		return ProcessResult{}, fmt.Errorf("error writing file: %w", err)
	}

	deps := findDependencies(content)
	return ProcessResult{
		FileInfo: router.FileInfo{
			ModTime:   item.Info.ModTime(),
			DistPath:  outPath,
			DependsOn: deps,
		},
		Content:      minified,
		Hash:         hashString,
		Dependencies: deps,
	}, nil
}

func findDependencies(content []byte) []string {
	var deps []string
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "import") || strings.Contains(line, "require") {
			if dep := extractDependency(line); dep != "" {
				deps = append(deps, dep)
			}
		}
	}
	return deps
}

func extractDependency(line string) string {
	line = strings.TrimSpace(line)
	if idx := strings.Index(line, "from"); idx != -1 {
		line = line[idx+4:]
	}
	line = strings.Trim(line, "'\"`;")
	if line != "" && !strings.HasPrefix(line, ".") {
		return line
	}
	return ""
}
