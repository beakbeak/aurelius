// Package media provides an HTTP API for exploring and streaming a library of
// audio media files.
package media

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/beakbeak/aurelius/pkg/textcache"
)

// LibraryConfig contains configuration parameters used by NewLibrary. It should
// be created with NewLibraryConfig to provide default values.
//
// Members without default values listed are required to be set by the user
// before passing to NewLibrary.
type LibraryConfig struct {
	RootPath string // The path to the media files in the local filesystem.

	// StoragePath is a directory in the local filesystem where persistent data
	// will be stored. If it does not exist, it will be created.
	StoragePath string

	// Prefix is the URL path prefix used for HTTP requests routed to the
	// Library. (Default: "/media")
	Prefix string

	// StreamAheadBytes controls how many encoded bytes beyond the current play
	// position to stream when streaming is throttled. The maximum of
	// StreamAheadBytes and StreamAheadTime will be used. (Default: 512KiB)
	StreamAheadBytes int

	// StreamAheadTime controls how far beyond the current play position to
	// stream when streaming is throttled. The maximum of StreamAheadBytes and
	// StreamAheadTime will be used. (Default: 10s)
	StreamAheadTime time.Duration

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
		StreamAheadBytes:  512 * 1024,
		StreamAheadTime:   10 * time.Second,
		ThrottleStreaming: true,
	}
}

// A Library provides an HTTP API for exploring and streaming a library of audio
// media files.
type Library struct {
	config        LibraryConfig
	playlistCache *textcache.TextCache
	handler       http.Handler
}

// NewLibrary creates a new Library object.
func NewLibrary(config *LibraryConfig) (*Library, error) {
	var err error

	if config.RootPath == "" {
		return nil, fmt.Errorf("RootPath is empty")
	}
	if config.StoragePath == "" {
		return nil, fmt.Errorf("StoragePath is empty")
	}

	if config.RootPath, err = filepath.EvalSymlinks(config.RootPath); err != nil {
		return nil, err
	}
	if info, err := os.Stat(config.RootPath); err != nil {
		return nil, err
	} else if !info.Mode().IsDir() {
		return nil, fmt.Errorf("not a directory: %v", config.RootPath)
	}

	if err := os.MkdirAll(config.StoragePath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create StoragePath: %v", err)
	}

	ml := Library{
		config:        *config,
		playlistCache: textcache.New(),
	}
	ml.setupHandler()

	slog.Info("media library opened", "prefix", config.Prefix, "root", config.RootPath)

	return &ml, nil
}

// ServeHTTP handles an HTTP request.
func (ml *Library) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	ml.handler.ServeHTTP(w, r)
}

func (ml *Library) setupHandler() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dirs/{dir}", makeHandler(ml, handleDirWrapper))
	mux.HandleFunc("GET /playlists/{playlist}", makeHandler(ml, handlePlaylistWrapper))
	mux.HandleFunc("GET /playlists/{playlist}/tracks/{track}", makeHandler(ml, handlePlaylistTrackWrapper))
	mux.HandleFunc("GET /tracks/{track}", makeHandler(ml, handleTrackWrapper))
	mux.HandleFunc("GET /tracks/{track}/stream", makeHandler(ml, handleTrackStreamWrapper))
	mux.HandleFunc("GET /tracks/{track}/images/{image}", makeHandler(ml, handleTrackImageWrapper))
	mux.HandleFunc("POST /tracks/{track}/favorite", makeHandler(ml, handleTrackFavoriteWrapper))
	mux.HandleFunc("POST /tracks/{track}/unfavorite", makeHandler(ml, handleTrackUnfavoriteWrapper))
	ml.handler = http.StripPrefix(ml.config.Prefix, mux)
}

func makeHandler(ml *Library, handlerFunc func(*Library, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handlerFunc(ml, w, r)
	}
}

var reAt = regexp.MustCompile(`^at:(.*)$`)

func parseAt(s string) (string, bool) {
	matches := reAt.FindStringSubmatch(s)
	if len(matches) > 0 {
		if path, err := url.PathUnescape(matches[1]); err == nil {
			return path, true
		}
	}
	return "", false
}

func handleDirWrapper(ml *Library, w http.ResponseWriter, r *http.Request) {
	if path, ok := parseAt(r.PathValue("dir")); ok {
		ml.handleDirInfoRequest(r.Context(), path, w)
	} else {
		http.NotFound(w, r)
	}
}

func handlePlaylistWrapper(ml *Library, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("playlist")
	if id == "favorites" {
		ml.handleFavoritesRequest(r.Context(), "info", w, r)
	} else if path, ok := parseAt(id); ok {
		ml.handlePlaylistRequest(r.Context(), path, "info", w, r)
	} else {
		http.NotFound(w, r)
	}
}

func handlePlaylistTrackWrapper(ml *Library, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("playlist")
	if id == "favorites" {
		ml.handleFavoritesRequest(r.Context(), r.PathValue("track"), w, r)
	} else if path, ok := parseAt(id); ok {
		ml.handlePlaylistRequest(r.Context(), path, r.PathValue("track"), w, r)
	} else {
		http.NotFound(w, r)
	}
}

func handleTrackWrapper(ml *Library, w http.ResponseWriter, r *http.Request) {
	if path, ok := parseAt(r.PathValue("track")); ok {
		ml.handleTrackRequest(r.Context(), path, "info", w, r)
	} else {
		http.NotFound(w, r)
	}
}

func handleTrackStreamWrapper(ml *Library, w http.ResponseWriter, r *http.Request) {
	if path, ok := parseAt(r.PathValue("track")); ok {
		ml.handleTrackRequest(r.Context(), path, "stream", w, r)
	} else {
		http.NotFound(w, r)
	}
}

func handleTrackImageWrapper(ml *Library, w http.ResponseWriter, r *http.Request) {
	if path, ok := parseAt(r.PathValue("track")); ok {
		ml.handleTrackImageRequest(r.Context(), path, r.PathValue("image"), w, r)
	} else {
		http.NotFound(w, r)
	}
}

func handleTrackFavoriteWrapper(ml *Library, w http.ResponseWriter, r *http.Request) {
	if path, ok := parseAt(r.PathValue("track")); ok {
		ml.handleTrackRequest(r.Context(), path, "favorite", w, r)
	} else {
		http.NotFound(w, r)
	}
}

func handleTrackUnfavoriteWrapper(ml *Library, w http.ResponseWriter, r *http.Request) {
	if path, ok := parseAt(r.PathValue("track")); ok {
		ml.handleTrackRequest(r.Context(), path, "unfavorite", w, r)
	} else {
		http.NotFound(w, r)
	}
}

func writeJson(
	ctx context.Context,
	w http.ResponseWriter,
	data interface{},
) {
	dataJson, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal JSON", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store")

	if _, err := w.Write(dataJson); err != nil {
		slog.ErrorContext(ctx, "failed to write response", "error", err)
	}
}
