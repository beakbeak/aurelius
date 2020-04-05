package database

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sb/aurelius/util"
	"time"
)

const (
	playAhead = 10000 * time.Millisecond
)

type Database struct {
	prefix        string
	root          string
	htmlPath      string
	playlistCache *FileCache

	throttleStreaming bool

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

	util.Debug.Printf("database opened: prefix='%v' root='%v'", prefix, rootPath)

	quotedPrefix := regexp.QuoteMeta(prefix)

	db := Database{
		prefix:        prefix,
		root:          rootPath,
		htmlPath:      htmlPath,
		playlistCache: NewFileCache(),

		throttleStreaming: true,

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

	util.Debug.Printf("DB request: %v\n", req.URL.Path)

	if matches := db.reDirPath.FindStringSubmatch(req.URL.Path); matches != nil {
		util.Debug.Println("dir request", matches)
		db.handleDirRequest(matches[2], w, req)
		return
	}

	if matches := db.rePlaylistPath.FindStringSubmatch(req.URL.Path); matches != nil {
		util.Debug.Println("playlist request", matches)
		db.handlePlaylistRequest(matches[1], w, req)
		return
	}

	if matches := db.reTrackPath.FindStringSubmatch(req.URL.Path); matches != nil {
		util.Debug.Println("track request", matches)
		db.handleTrackRequest(matches[1], matches[2], w, req)
		return
	}

	http.NotFound(w, req)
}
