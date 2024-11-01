package fileaccess

import (
	"io"
	"os"
)

// FileAccess provides direct file operations
type FileAccess struct{}

// New creates a FileAccess instance
func New() *FileAccess {
	return &FileAccess{}
}

// Read reads entire file content
func (fa *FileAccess) Read(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Get file size
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := stat.Size()
	buf := make([]byte, size)
	_, err = io.ReadFull(f, buf)
	return buf, err
}

// Open returns a ReadSeekCloser for streaming operations
func (fa *FileAccess) Open(path string) (*os.File, error) {
	return os.Open(path)
}

// Stat returns file information
func (fa *FileAccess) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
