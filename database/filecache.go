package database

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

type cachedFile struct {
	ModTime time.Time
	Lines   []string
}

type FileCache struct {
	mutex sync.Mutex
	files map[string]*cachedFile
}

func NewFileCache() *FileCache {
	return &FileCache{files: make(map[string]*cachedFile)}
}

func (c *FileCache) Get(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat failed: %v", err)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if cached, ok := c.files[path]; ok {
		if cached.ModTime.Equal(info.ModTime()) || cached.ModTime.After(info.ModTime()) {
			return cached.Lines, nil
		}
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cached := cachedFile{ModTime: info.ModTime()}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		cached.Lines = append(cached.Lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan failed: %v", err)
	}

	c.files[path] = &cached
	return cached.Lines, nil
}
