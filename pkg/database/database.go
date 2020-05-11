package database

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sb/aurelius/pkg/textcache"
	"time"
)

const (
	playAhead = 10000 * time.Millisecond
)

type Database struct {
	prefix        string
	root          string
	htmlPath      string
	playlistCache *textcache.TextCache

	throttleStreaming      bool
	deterministicStreaming bool

	reDirPath      *regexp.Regexp
	rePlaylistPath *regexp.Regexp
	reTrackPath    *regexp.Regexp
}

func New(
	prefix string,
	rootPath string,
	htmlPath string,
) (*Database, error) {
	var err error
	if rootPath, err = filepath.EvalSymlinks(rootPath); err != nil {
		return nil, err
	}
	if info, err := os.Stat(rootPath); err != nil {
		return nil, err
	} else if !info.Mode().IsDir() {
		return nil, fmt.Errorf("not a directory: %v", rootPath)
	}

	logger(LogDebug).Printf("database opened: prefix='%v' root='%v'", prefix, rootPath)

	quotedPrefix := regexp.QuoteMeta(prefix)

	db := Database{
		prefix:        prefix,
		root:          rootPath,
		htmlPath:      htmlPath,
		playlistCache: textcache.New(),

		throttleStreaming:      true,
		deterministicStreaming: false,

		reDirPath:      regexp.MustCompile(`^` + quotedPrefix + `/((.*?)/)?$`),
		rePlaylistPath: regexp.MustCompile(`^` + quotedPrefix + `/((?i).+?\.m3u)$`),
		reTrackPath:    regexp.MustCompile(`^` + quotedPrefix + `/(.+?)/([^/]+)$`),
	}
	return &db, nil
}

func (db *Database) Prefix() string {
	return db.prefix
}

func (db *Database) ServeHTTP(
	w http.ResponseWriter,
	req *http.Request,
) {
	if req.URL.Path == db.prefix {
		http.Redirect(w, req, db.prefix+"/", http.StatusFound)
		return
	}

	logger(LogDebug).Printf("DB request: %v\n", req.URL.Path)

	if matches := db.reDirPath.FindStringSubmatch(req.URL.Path); matches != nil {
		logger(LogDebug).Println("dir request", matches)
		db.handleDirRequest(matches[2], w, req)
		return
	}

	if matches := db.rePlaylistPath.FindStringSubmatch(req.URL.Path); matches != nil {
		logger(LogDebug).Println("playlist request", matches)
		db.handlePlaylistRequest(matches[1], w, req)
		return
	}

	if matches := db.reTrackPath.FindStringSubmatch(req.URL.Path); matches != nil {
		logger(LogDebug).Println("track request", matches)
		db.handleTrackRequest(matches[1], matches[2], w, req)
		return
	}

	http.NotFound(w, req)
}

func writeJson(
	w http.ResponseWriter,
	data interface{},
) {
	dataJson, err := json.Marshal(data)
	if err != nil {
		logger(LogDebug).Printf("failed to marshal JSON: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store")

	if _, err := w.Write(dataJson); err != nil {
		logger(LogDebug).Printf("failed to write response: %v\n", err)
	}
}
