// Package textcache provides a thread-safe read/write text file cache. Accessed
// files are mirrored fully in memory.
package textcache

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

type TextCache struct {
	mutex sync.Mutex
	files map[string]*cachedFile
}

type cachedFile struct {
	ModTime time.Time
	Lines   []string
}

// New creates a new TextCache.
func New() *TextCache {
	return &TextCache{files: make(map[string]*cachedFile)}
}

// Get returns the contents of a file as an array of lines.
// Newline characters are stripped.
func (c *TextCache) Get(path string) ([]string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.unsafeGet(path)
}

func (c *TextCache) unsafeGet(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err // not reformatting because caller may call os.IsNotExist(err)
	}

	return c.unsafeGetWithInfo(path, info)
}

func (c *TextCache) unsafeGetWithInfo(
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

// Write joins lines with "\n" and writes them to the file specified by path.
func (c *TextCache) Write(
	path string,
	lines []string,
) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.unsafeWrite(path, lines)
}

func (c *TextCache) unsafeWrite(
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

// Modify combines the functionality of Get and Write. It passes the contents of
// a file as an array of lines to modifier and writes the returned result to
// path, joining the strings with "\n".
//
// An error will be returned if the file does not exist. If modifier returns
// nil, the file will not be written.
func (c *TextCache) Modify(
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

// CreateOrModify performs the same operation as Modify, but will create the
// file if it does not exist. It passes the contents of a file as an array of
// lines to modifier and writes the returned result to path, joining the strings
// with "\n".
//
// If modifier returns nil, the file will not be written.
func (c *TextCache) CreateOrModify(
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

func (c *TextCache) modifyImpl(
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
