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

func (ml *Library) handleTrackRequest(
	libraryPath string,
	resource string,
	w http.ResponseWriter,
	req *http.Request,
) {
	fsPath := ml.toFileSystemPath(libraryPath)
	if info, err := os.Stat(fsPath); err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, req)
		return
	}

	handled := true

	switch req.Method {
	case http.MethodGet:
		switch resource {
		case "stream":
			ml.handleTrackStreamRequest(fsPath, w, req)
		case "info":
			ml.handleTrackInfoRequest(libraryPath, fsPath, w, req)
		default:
			handled = false
		}

	case http.MethodPost:
		switch resource {
		case "favorite":
			if err := ml.setFavorite(libraryPath, true); err != nil {
				logger(LogDebug).Printf("Favorite failed: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				writeJson(w, nil)
			}

		case "unfavorite":
			if err := ml.setFavorite(libraryPath, false); err != nil {
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

func (ml *Library) handleTrackInfoRequest(
	urlPath string,
	fsPath string,
	w http.ResponseWriter,
	req *http.Request,
) {
	src, err := newAudioSource(fsPath)
	if err != nil {
		logger(LogDebug).Printf("failed to open source '%v': %v\n", fsPath, err)
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
		Name:            filepath.Base(fsPath),
		Duration:        float64(src.Duration()) / float64(time.Second),
		ReplayGainTrack: src.ReplayGain(aurelib.ReplayGainTrack, true),
		ReplayGainAlbum: src.ReplayGain(aurelib.ReplayGainAlbum, true),
		Tags:            lowerCaseKeys(src.Tags()),
	}

	if favorite, err := ml.isFavorite(urlPath); err != nil {
		logger(LogDebug).Printf("isFavorite failed: %v", err)
	} else {
		result.Favorite = favorite
	}

	writeJson(w, result)
}

func newAudioSource(path string) (aurelib.Source, error) {
	if fragment.IsFragment(path) {
		return fragment.New(path)
	}
	return aurelib.NewFileSource(path)
}

func (ml *Library) isFavorite(path string) (bool, error) {
	favorites, err := ml.playlistCache.Get(ml.toFileSystemPath(favoritesPath))
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

func (ml *Library) setFavorite(
	path string,
	favorite bool,
) error {
	if favorite {
		return ml.playlistCache.CreateOrModify(
			ml.toFileSystemPath(favoritesPath),
			func(favorites []string) ([]string, error) {
				for _, line := range favorites {
					if line == path {
						return nil, nil
					}
				}
				return append(favorites, path), nil
			},
		)
	} else {
		return ml.playlistCache.CreateOrModify(
			ml.toFileSystemPath(favoritesPath),
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
