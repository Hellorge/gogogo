package fileaccess

import (
	"io"
	"os"
	"sync"
)

// FileAccess provides direct file operations
type FileAccess struct {
    // Buffer pool for small file reads
    bufferPool sync.Pool
}

// New creates a FileAccess instance
func New() *FileAccess {
    return &FileAccess{
        bufferPool: sync.Pool{
            New: func() interface{} {
                return make([]byte, 32*1024) // 32KB default buffer
            },
        },
    }
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

    // For small files, use buffer pool
    if size := stat.Size(); size <= 32*1024 {
        buf := fa.bufferPool.Get().([]byte)
        buf = buf[:cap(buf)]
        defer fa.bufferPool.Put(buf)

        n, err := f.Read(buf)
        if err != nil && err != io.EOF {
            return nil, err
        }

        // Create copy of exactly the right size
        result := make([]byte, n)
        copy(result, buf[:n])
        return result, nil
    }

    // For large files, allocate exactly the right size
    buf := make([]byte, stat.Size())
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
