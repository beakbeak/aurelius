package database

import (
	"fmt"
	"io/ioutil"
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

	reDirPath  *regexp.Regexp
	reFilePath *regexp.Regexp
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
		root:          rootPath,
		templateProxy: templateProxy,
	}

	var err error
	if db.reDirPath, err = regexp.Compile(`^` + prefix + `/(.*)$`); err != nil {
		return nil, err
	}
	if db.reFilePath, err = regexp.Compile(`^` + prefix + `/(.+?)/([^/]+)$`); err != nil {
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

func (db *Database) handleFileRequest(
	w http.ResponseWriter,
	req *http.Request,
) (handled bool, _ error) {
	groups := db.reFilePath.FindStringSubmatch(req.URL.Path)
	if groups == nil {
		return false, nil
	}

	path := db.expandPath(groups[1])
	subRequest := groups[2]

	{
		info, err := os.Stat(path)
		if err != nil {
			return false, nil
		}

		mode := info.Mode()
		if mode.IsDir() {
			return false, nil
		}
		if !mode.IsRegular() {
			return false, fmt.Errorf("not a symlink or regular file: %v", path)
		}
	}

	switch subRequest {
	case "stream":
		util.Noise.Printf("stream %v\n", path)
		db.Stream(path, w, req)
	default:
		return false, fmt.Errorf("invalid DB request: %v", subRequest)
	}
	return true, nil
}

func (db *Database) handleDirRequest(
	w http.ResponseWriter,
	req *http.Request,
) (handled bool, _ error) {
	groups := db.reDirPath.FindStringSubmatch(req.URL.Path)
	if groups == nil {
		return false, nil
	}

	path := db.expandPath(groups[1])
	infos, err := ioutil.ReadDir(path)
	if err != nil {
		return false, err
	}

	type TemplateData struct {
		Dirs  []string
		Files []string
	}
	data := TemplateData{}

	for _, info := range infos {
		mode := info.Mode()
		if mode.IsDir() {
			data.Dirs = append(data.Dirs, info.Name())
		} else if mode.IsRegular() {
			data.Files = append(data.Files, info.Name())
		}
		// TODO: handle symlinks
	}

	if err := db.templateProxy.ExecuteTemplate(w, "db-dir.html", data); err != nil {
		util.Debug.Printf("failed to execute template: %v\n", err)
	}
	return true, nil
}

func (db *Database) HandleRequest(
	w http.ResponseWriter,
	req *http.Request,
) {
	if req.URL.Path == db.prefix {
		http.Redirect(w, req, db.prefix+"/", http.StatusFound)
		return
	}

	reject := func(format string, args ...interface{}) {
		w.WriteHeader(http.StatusNotFound)
		util.Debug.Printf(format, args...)
	}

	util.Debug.Printf("DB request: %v\n", req.URL.Path)

	handled, err := db.handleFileRequest(w, req)
	if err != nil {
		reject("file request failed: %v\n", err)
		return
	}
	if handled {
		return
	}

	handled, err = db.handleDirRequest(w, req)
	if err != nil {
		reject("directory request failed: %v\n", err)
		return
	}
	if handled {
		return
	}

	util.Debug.Printf("unhandled DB request: %v\n", req.URL.Path)
}
