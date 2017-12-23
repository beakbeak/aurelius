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

func (c *FileCache) unsafeGetWithInfo(
	path string,
	info os.FileInfo,
) ([]string, error) {
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

	cached := cachedFile{ModTime: info.ModTime(), Lines: []string{}}
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

func (c *FileCache) unsafeGet(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat failed: %v", err)
	}

	return c.unsafeGetWithInfo(path, info)
}

func (c *FileCache) Get(path string) ([]string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.unsafeGet(path)
}

func (c *FileCache) unsafeWrite(
	path string,
	lines []string,
) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		_, err := writer.WriteString(line)
		if err != nil {
			return err
		}
		_, err = writer.WriteString("\n")
		if err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	info, err := file.Stat()
	if err != nil {
		return err
	}

	cached := cachedFile{ModTime: info.ModTime(), Lines: lines}
	c.files[path] = &cached
	return nil
}

func (c *FileCache) Write(
	path string,
	lines []string,
) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.unsafeWrite(path, lines)
}

func (c *FileCache) modifyImpl(
	path string,
	modifier func(lines []string) ([]string, error),
	lines []string,
) error {
	lines, err := modifier(lines)
	if err != nil {
		return err
	}
	if lines == nil {
		return nil
	}

	return c.unsafeWrite(path, lines)
}

// returning nil from modifier will result in no change
func (c *FileCache) Modify(
	path string,
	modifier func(lines []string) ([]string, error),
) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	lines, err := c.unsafeGet(path)
	if err != nil {
		return err
	}

	return c.modifyImpl(path, modifier, lines)
}

// returning nil from modifier will result in no change
func (c *FileCache) CreateOrModify(
	path string,
	modifier func(lines []string) ([]string, error),
) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var lines []string

	info, err := os.Stat(path)
	if err == nil {
		if lines, err = c.unsafeGetWithInfo(path, info); err != nil {
			return err
		}
	} else {
		// XXX some error types should cause a `return err` here
		lines = make([]string, 0)
	}

	return c.modifyImpl(path, modifier, lines)
}
