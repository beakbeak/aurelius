package media

import (
	"net/http"
	"os"
	"path/filepath"
	"sb/aurelius/internal/maputil"
	"sb/aurelius/pkg/aurelib"
	"sb/aurelius/pkg/fragment"
	"time"
)

func (ml *Library) handleTrackRequest(
	libraryPath string,
	resource string,
	w http.ResponseWriter,
	req *http.Request,
) {
	fsPath := ml.libraryToFsPath(libraryPath)
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
				log.Printf("Favorite failed: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				writeJson(w, nil)
			}

		case "unfavorite":
			if err := ml.setFavorite(libraryPath, false); err != nil {
				log.Printf("Unfavorite failed: %v\n", err)
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
		log.Printf("failed to open source '%v': %v\n", fsPath, err)
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
		Tags:            maputil.LowerCaseKeys(src.Tags()),
	}

	if favorite, err := ml.isFavorite(urlPath); err != nil {
		log.Printf("isFavorite failed: %v", err)
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
	favorites, err := ml.playlistCache.Get(ml.storageToFsPath(favoritesPath))
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
			ml.storageToFsPath(favoritesPath),
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
			ml.storageToFsPath(favoritesPath),
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
