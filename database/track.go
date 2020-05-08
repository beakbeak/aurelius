package database

import (
	"net/http"
	"os"
	"path/filepath"
	"sb/aurelius/aurelib"
	"sb/aurelius/fragment"
	"sb/aurelius/internal/util"
	"time"
)

const favoritesPath = "Favorites.m3u"

func (db *Database) handleTrackRequest(
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
				util.Debug.Printf("Favorite failed: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				util.WriteJson(w, nil)
			}
		case "unfavorite":
			if err := db.Unfavorite(urlPath); err != nil {
				util.Debug.Printf("Unfavorite failed: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				util.WriteJson(w, nil)
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

func (db *Database) IsFavorite(path string) (bool, error) {
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

func (db *Database) handleInfoRequest(
	urlPath string,
	filePath string,
	w http.ResponseWriter,
	req *http.Request,
) {
	src, err := newAudioSource(filePath)
	if err != nil {
		util.Debug.Printf("failed to open source '%v': %v\n", filePath, err)
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
		Tags:            util.LowerCaseKeys(src.Tags()),
	}

	if favorite, err := db.IsFavorite(urlPath); err != nil {
		util.Debug.Printf("isFavorite failed: %v", err)
	} else {
		result.Favorite = favorite
	}

	util.WriteJson(w, result)
}

func (db *Database) Favorite(path string) error {
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

func (db *Database) Unfavorite(path string) error {
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
