// Package media provides an HTTP API for exploring and streaming a library of
// audio media files.
package media

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sb/aurelius/pkg/textcache"
	"time"
)

// LibraryConfig contains configuration parameters used by NewLibrary. It should
// be created with NewLibraryConfig to provide default values.
type LibraryConfig struct {
	RootPath string // The path to the media files in the local filesystem.

	// Prefix is the URL path prefix used for HTTP requests routed to the
	// Library. (Default: "/media")
	Prefix string

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

// NewLibraryConfig creates a new LibraryConfig object with default values.
func NewLibraryConfig() *LibraryConfig {
	return &LibraryConfig{
		Prefix:            "/media",
		PlayAhead:         10 * time.Second,
		ThrottleStreaming: true,
	}
}

// A Library provides an HTTP API for exploring and streaming a library of audio
// media files.
type Library struct {
	config LibraryConfig

	playlistCache *textcache.TextCache

	reRootPath         *regexp.Regexp
	reDirPath          *regexp.Regexp
	reFileResourcePath *regexp.Regexp
	rePlaylistPath     *regexp.Regexp
}

// NewLibrary creates a new Library object.
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
	quotedTreePrefix := regexp.QuoteMeta(path.Join(config.Prefix, treePrefix))

	ml := Library{
		config: *config,

		playlistCache: textcache.New(),

		reRootPath:         regexp.MustCompile(`^(` + quotedPrefix + `/?|` + quotedTreePrefix + `)$`),
		reDirPath:          regexp.MustCompile(`^` + quotedTreePrefix + `/((.*?)/)?$`),
		reFileResourcePath: regexp.MustCompile(`^` + quotedTreePrefix + `/(.+?)/([^/]+)$`),
		rePlaylistPath:     regexp.MustCompile(`^(?i).+?\.m3u$`),
	}

	log.Printf(
		"media library opened: prefix='%v' root='%v'", config.Prefix, config.RootPath)

	return &ml, nil
}

// ServeHTTP handles an HTTP request. It returns true if the request was
// handled, and false if a directory was requested without a query string. In
// this case, the caller should handle the request by serving HTML for the user
// interface.
func (ml *Library) ServeHTTP(
	w http.ResponseWriter,
	req *http.Request,
) bool {
	if ml.reRootPath.MatchString(req.URL.Path) {
		http.Redirect(w, req, ml.libraryToUrlPath("")+"/", http.StatusFound)
		return true
	}

	if matches := ml.reDirPath.FindStringSubmatch(req.URL.Path); matches != nil {
		libraryPath := matches[2]

		return ml.handleDirRequest(libraryPath, w, req)
	}
	if matches := ml.reFileResourcePath.FindStringSubmatch(req.URL.Path); matches != nil {
		libraryPath := matches[1]
		resource := matches[2]

		if ml.rePlaylistPath.FindStringSubmatch(libraryPath) != nil {
			ml.handlePlaylistRequest(libraryPath, resource, w, req)
			return true
		}
		ml.handleTrackRequest(libraryPath, resource, w, req)
		return true
	}

	http.NotFound(w, req)
	return true
}

func writeJson(
	w http.ResponseWriter,
	data interface{},
) {
	dataJson, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal JSON: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store")

	if _, err := w.Write(dataJson); err != nil {
		log.Printf("failed to write response: %v\n", err)
	}
}
