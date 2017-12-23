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
	playAhead = 10000 * time.Millisecond // TODO: make configurable
)

type Database struct {
	prefix        string
	root          string
	templateProxy util.TemplateProxy
	playlistCache *FileCache

	reDirPath      *regexp.Regexp
	rePlaylistPath *regexp.Regexp
	reTrackPath    *regexp.Regexp
	reIgnore       *regexp.Regexp
	rePlaylist     *regexp.Regexp
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

	var err error
	if db.reDirPath, err = regexp.Compile(`^` + prefix + `/(:?(.*?)/)?$`); err != nil {
		return nil, err
	}
	if db.rePlaylistPath, err = regexp.Compile(`^` + prefix + `/(.+?\.[mM]3[uU])$`); err != nil {
		return nil, err
	}
	if db.reTrackPath, err = regexp.Compile(`^` + prefix + `/(.+?)/([^/]+)$`); err != nil {
		return nil, err
	}
	if db.reIgnore, err = regexp.Compile(
		`(?i)\.(:?jpe?g|png|txt|log|cue|gif|pdf|sfv|nfo)$`,
	); err != nil {
		return nil, err
	}
	if db.rePlaylist, err = regexp.Compile(`\.[mM]3[uU]`); err != nil {
		return nil, err
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

	if matches := db.reDirPath.FindStringSubmatch(req.URL.Path); matches != nil {
		util.Debug.Println("dir request")
		db.handleDirRequest(matches, w, req)
		return
	}

	if matches := db.rePlaylistPath.FindStringSubmatch(req.URL.Path); matches != nil {
		util.Debug.Println("playlist request")
		db.handlePlaylistRequest(matches, w, req)
		return
	}

	if matches := db.reTrackPath.FindStringSubmatch(req.URL.Path); matches != nil {
		util.Debug.Println("track request")
		db.handleTrackRequest(matches, w, req)
		return
	}

	http.NotFound(w, req)
}
