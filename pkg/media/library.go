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

type LibraryConfig struct {
	Prefix   string
	RootPath string
	HtmlPath string

	// PlayAhead controls how far beyond the current play position to stream
	// when streaming is throttled. (Default: 10s)
	PlayAhead time.Duration

	// ThrottleStreaming controls whether streaming throughput is limited to
	// playback speed. If false, streaming throughput is not limited.
	// (Default: true)
	ThrottleStreaming bool

	// DeterministicStreaming controls whether to avoid randomness in encoding
	// and muxing. It should be set to true when deterministic output is needed,
	// such as when performing automated testing. (Default: false)
	DeterministicStreaming bool
}

func NewLibraryConfig() *LibraryConfig {
	return &LibraryConfig{
		Prefix:            "/media",
		PlayAhead:         10 * time.Second,
		ThrottleStreaming: true,
	}
}

type Library struct {
	config LibraryConfig

	playlistCache *textcache.TextCache

	reDirPath      *regexp.Regexp
	rePlaylistPath *regexp.Regexp
	reTrackPath    *regexp.Regexp
}

func NewLibrary(config *LibraryConfig) (*Library, error) {
	var err error

	if config.RootPath == "" {
		return nil, fmt.Errorf("RootPath is empty")
	}
	if config.RootPath, err = filepath.EvalSymlinks(config.RootPath); err != nil {
		return nil, err
	}
	if info, err := os.Stat(config.RootPath); err != nil {
		return nil, err
	} else if !info.Mode().IsDir() {
		return nil, fmt.Errorf("not a directory: %v", config.RootPath)
	}

	quotedPrefix := regexp.QuoteMeta(config.Prefix)
	ml := Library{
		config: *config,

		playlistCache: textcache.New(),

		reDirPath:      regexp.MustCompile(`^` + quotedPrefix + `/((.*?)/)?$`),
		rePlaylistPath: regexp.MustCompile(`^` + quotedPrefix + `/((?i).+?\.m3u)$`),
		reTrackPath:    regexp.MustCompile(`^` + quotedPrefix + `/(.+?)/([^/]+)$`),
	}

	logger(LogDebug).Printf(
		"media library opened: prefix='%v' root='%v'", config.Prefix, config.RootPath)

	return &ml, nil
}

func (ml *Library) ServeHTTP(
	w http.ResponseWriter,
	req *http.Request,
) {
	if req.URL.Path == ml.config.Prefix {
		http.Redirect(w, req, ml.config.Prefix+"/", http.StatusFound)
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
