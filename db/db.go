package db

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sb/aurelius/aurelog"
	"time"
)

// TODO: disallow looking outside DB root!

const (
	Prefix    = "/db/"
	playAhead = 10000 * time.Millisecond // TODO: make configurable
)

var (
	reDirPath  *regexp.Regexp
	reFilePath *regexp.Regexp

	tmplDir *template.Template

	tmplDirStr = `<!DOCTYPE html>
<html>
<body>
{{if .Dirs}}
{{range $dir := .Dirs}}<a href="{{$dir}}">{{$dir}}</a>/<br/>{{end}}
<br/>
{{end}}
{{if .Files}}
{{range $file := .Files}}<a href="{{$file}}/stream">{{$file}}</a><br/>{{end}}
{{end}}
</body>
</html>`
)

func init() {
	var err error
	if reDirPath, err = regexp.Compile(`^` + Prefix + `(.*)$`); err != nil {
		panic(err)
	}
	if reFilePath, err = regexp.Compile(`^` + Prefix + `(.+?)/([^/]+)$`); err != nil {
		panic(err)
	}

	tmplDir, err = template.New("dir").Parse(tmplDirStr)
	if err != nil {
		panic(err)
	}
}

type Database struct {
	root string
}

func NewDatabase(rootPath string) (*Database, error) {
	rootPath = filepath.Clean(rootPath)
	if info, err := os.Stat(rootPath); err != nil {
		return nil, err
	} else if !info.Mode().IsDir() {
		return nil, fmt.Errorf("not a directory: %v", rootPath)
	}

	return &Database{root: rootPath}, nil
}

func (db *Database) expandPath(path string) string {
	return filepath.Join(db.root, path)
}

func (db *Database) handleFileRequest(
	w http.ResponseWriter,
	req *http.Request,
) (handled bool, _ error) {
	groups := reFilePath.FindStringSubmatch(req.URL.Path)
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
		aurelog.Noise.Printf("stream %v\n", path)
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
	groups := reDirPath.FindStringSubmatch(req.URL.Path)
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

	tmplDir.Execute(w, data)
	return true, nil
}

func (db *Database) HandleRequest(
	w http.ResponseWriter,
	req *http.Request,
) {
	reject := func(format string, args ...interface{}) {
		w.WriteHeader(http.StatusNotFound)
		aurelog.Debug.Printf(format, args...)
	}

	aurelog.Debug.Printf("DB request: %v\n", req.URL.Path)

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

	aurelog.Debug.Printf("unhandled DB request: %v\n", req.URL.Path)
}
