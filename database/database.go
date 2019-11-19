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
	prefix    = "/db"
	playAhead = 10000 * time.Millisecond
)

var (
	reDirPath      *regexp.Regexp
	rePlaylistPath *regexp.Regexp
	reTrackPath    *regexp.Regexp
)

func init() {
	var err error
	if reDirPath, err = regexp.Compile(`^` + prefix + `/(:?(.*?)/)?$`); err != nil {
		panic(err)
	}
	if rePlaylistPath, err = regexp.Compile(`^` + prefix + `/(.+?\.[mM]3[uU])$`); err != nil {
		panic(err)
	}
	if reTrackPath, err = regexp.Compile(`^` + prefix + `/(.+?)/([^/]+)$`); err != nil {
		panic(err)
	}
}

type Database struct {
	prefix        string
	root          string
	templateProxy util.TemplateProxy
	playlistCache *FileCache
}

func New(
	prefix string,
	rootPath string,
	templateProxy util.TemplateProxy,
) (*Database, error) {
	rootPath = filepath.Clean(rootPath)
	if info, err := os.Stat(rootPath); err != nil {
		return nil, err
	} else if !info.Mode().IsDir() {
		return nil, fmt.Errorf("not a directory: %v", rootPath)
	}

	db := Database{
		prefix:        prefix,
		root:          rootPath,
		templateProxy: templateProxy,
		playlistCache: NewFileCache(),
	}
	return &db, nil
}

func (db *Database) Prefix() string {
	return db.prefix
}

func (db *Database) expandPath(path string) string {
	return filepath.Join(db.root, path)
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

	if matches := reDirPath.FindStringSubmatch(req.URL.Path); matches != nil {
		util.Debug.Println("dir request")
		db.handleDirRequest(matches, w, req)
		return
	}

	if matches := rePlaylistPath.FindStringSubmatch(req.URL.Path); matches != nil {
		util.Debug.Println("playlist request")
		db.handlePlaylistRequest(matches, w, req)
		return
	}

	if matches := reTrackPath.FindStringSubmatch(req.URL.Path); matches != nil {
		util.Debug.Println("track request")
		db.handleTrackRequest(matches, w, req)
		return
	}

	http.NotFound(w, req)
}
