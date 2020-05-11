package media

import (
	"net/http"
	"os"
	"path/filepath"
	"sb/aurelius/pkg/aurelib"
	"sb/aurelius/pkg/fragment"
	"strings"
	"time"
)

const favoritesPath = "Favorites.m3u"

func (db *Library) handleTrackRequest(
	urlPath string,
	subRequest string,
	w http.ResponseWriter,
	req *http.Request,
) {
	filePath := db.toFileSystemPath(urlPath)
	if info, err := os.Stat(filePath); err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, req)
		return
	}

	handled := true

	switch req.Method {
	case http.MethodGet:
		switch subRequest {
		case "stream":
			db.handleStreamRequest(filePath, w, req)
		case "info":
			db.handleInfoRequest(urlPath, filePath, w, req)
		default:
			handled = false
		}
	case http.MethodPost:
		switch subRequest {
		case "favorite":
			if err := db.Favorite(urlPath); err != nil {
				logger(LogDebug).Printf("Favorite failed: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				writeJson(w, nil)
			}
		case "unfavorite":
			if err := db.Unfavorite(urlPath); err != nil {
				logger(LogDebug).Printf("Unfavorite failed: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				writeJson(w, nil)
			}
		default:
			handled = false
		}
	default:
		handled = false
	}

	if !handled {
		http.NotFound(w, req)
	}
}

func (db *Library) IsFavorite(path string) (bool, error) {
	favorites, err := db.playlistCache.Get(db.toFileSystemPath(favoritesPath))
	switch {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return false, err
	}

	for _, line := range favorites {
		if line == path {
			return true, nil
		}
	}
	return false, nil
}

func newAudioSource(path string) (aurelib.Source, error) {
	if fragment.IsFragment(path) {
		return fragment.New(path)
	}
	return aurelib.NewFileSource(path)
}

func filterKeys(
	data map[string]string,
	filter func(string) (string, bool),
) map[string]string {
	out := make(map[string]string, len(data))
	for key, value := range data {
		if outKey, ok := filter(key); ok {
			out[outKey] = value
		}
	}
	return out
}

func lowerCaseKeys(data map[string]string) map[string]string {
	return filterKeys(data, func(s string) (string, bool) {
		return strings.ToLower(s), true
	})
}

func (db *Library) handleInfoRequest(
	urlPath string,
	filePath string,
	w http.ResponseWriter,
	req *http.Request,
) {
	src, err := newAudioSource(filePath)
	if err != nil {
		logger(LogDebug).Printf("failed to open source '%v': %v\n", filePath, err)
		http.NotFound(w, req)
		return
	}
	defer src.Destroy()

	type Result struct {
		Name            string            `json:"name"`
		Duration        float64           `json:"duration"`
		ReplayGainTrack float64           `json:"replayGainTrack"`
		ReplayGainAlbum float64           `json:"replayGainAlbum"`
		Favorite        bool              `json:"favorite"`
		Tags            map[string]string `json:"tags"`
	}

	result := Result{
		Name:            filepath.Base(filePath),
		Duration:        float64(src.Duration()) / float64(time.Second),
		ReplayGainTrack: src.ReplayGain(aurelib.ReplayGainTrack, true),
		ReplayGainAlbum: src.ReplayGain(aurelib.ReplayGainAlbum, true),
		Tags:            lowerCaseKeys(src.Tags()),
	}

	if favorite, err := db.IsFavorite(urlPath); err != nil {
		logger(LogDebug).Printf("isFavorite failed: %v", err)
	} else {
		result.Favorite = favorite
	}

	writeJson(w, result)
}

func (db *Library) Favorite(path string) error {
	return db.playlistCache.CreateOrModify(
		db.toFileSystemPath(favoritesPath),
		func(favorites []string) ([]string, error) {
			for _, line := range favorites {
				if line == path {
					return nil, nil
				}
			}
			return append(favorites, path), nil
		},
	)
}

func (db *Library) Unfavorite(path string) error {
	return db.playlistCache.CreateOrModify(
		db.toFileSystemPath(favoritesPath),
		func(favorites []string) ([]string, error) {
			for index, line := range favorites {
				if line == path {
					return append(favorites[:index], favorites[index+1:]...), nil
				}
			}
			return nil, nil
		},
	)
}
