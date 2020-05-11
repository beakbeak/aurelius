package media

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

type Library struct {
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

func NewLibrary(
	prefix string,
	rootPath string,
	htmlPath string,
) (*Library, error) {
	var err error
	if rootPath, err = filepath.EvalSymlinks(rootPath); err != nil {
		return nil, err
	}
	if info, err := os.Stat(rootPath); err != nil {
		return nil, err
	} else if !info.Mode().IsDir() {
		return nil, fmt.Errorf("not a directory: %v", rootPath)
	}

	logger(LogDebug).Printf("media library opened: prefix='%v' root='%v'", prefix, rootPath)

	quotedPrefix := regexp.QuoteMeta(prefix)

	ml := Library{
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
	return &ml, nil
}

func (ml *Library) Prefix() string {
	return ml.prefix
}

func (ml *Library) ServeHTTP(
	w http.ResponseWriter,
	req *http.Request,
) {
	if req.URL.Path == ml.prefix {
		http.Redirect(w, req, ml.prefix+"/", http.StatusFound)
		return
	}

	logger(LogDebug).Printf("media request: %v\n", req.URL.Path)

	if matches := ml.reDirPath.FindStringSubmatch(req.URL.Path); matches != nil {
		logger(LogDebug).Println("dir request", matches)
		ml.handleDirRequest(matches[2], w, req)
		return
	}

	if matches := ml.rePlaylistPath.FindStringSubmatch(req.URL.Path); matches != nil {
		logger(LogDebug).Println("playlist request", matches)
		ml.handlePlaylistRequest(matches[1], w, req)
		return
	}

	if matches := ml.reTrackPath.FindStringSubmatch(req.URL.Path); matches != nil {
		logger(LogDebug).Println("track request", matches)
		ml.handleTrackRequest(matches[1], matches[2], w, req)
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
