package media

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/beakbeak/aurelius/internal/maputil"
	"github.com/beakbeak/aurelius/pkg/aurelib"
	"github.com/beakbeak/aurelius/pkg/fragment"
)

const (
	cachedImageMaxAge = 3600 * time.Second
)

func findDirectoryImage(trackPath string) *aurelib.AttachedImage {
	dir := filepath.Dir(trackPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	knownImageExts := []string{".jpg", ".jpeg", ".png", ".gif"}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !slices.Contains(knownImageExts, ext) {
			continue
		}
		imagePath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(imagePath)
		if err != nil {
			continue
		}
		var format aurelib.AttachedImageFormat
		switch ext {
		case ".jpg", ".jpeg":
			format = aurelib.AttachedImageJPEG
		case ".png":
			format = aurelib.AttachedImagePNG
		case ".gif":
			format = aurelib.AttachedImageGIF
		}
		return &aurelib.AttachedImage{
			Data:   data,
			Format: format,
		}
	}
	return nil
}

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

	type AttachedImageInfo struct {
		Format string `json:"format"`
		Hash   string `json:"hash"`
	}

	type Result struct {
		Name            string              `json:"name"`
		Duration        float64             `json:"duration"`
		ReplayGainTrack float64             `json:"replayGainTrack"`
		ReplayGainAlbum float64             `json:"replayGainAlbum"`
		Favorite        bool                `json:"favorite"`
		Tags            map[string]string   `json:"tags"`
		AttachedImages  []AttachedImageInfo `json:"attachedImages"`
	}

	attachedImageToInfo := func(image aurelib.AttachedImage) AttachedImageInfo {
		var formatStr string
		switch image.Format {
		case aurelib.AttachedImageJPEG:
			formatStr = "JPEG"
		case aurelib.AttachedImagePNG:
			formatStr = "PNG"
		case aurelib.AttachedImageGIF:
			formatStr = "GIF"
		}

		hash := sha256.Sum256(image.Data)
		hashStr := hex.EncodeToString(hash[:])

		return AttachedImageInfo{
			Format: formatStr,
			Hash:   hashStr,
		}
	}

	srcImages := src.AttachedImages()
	attachedImages := make([]AttachedImageInfo, 0, len(srcImages))

	if len(srcImages) == 0 {
		if dirImage := findDirectoryImage(fsPath); dirImage != nil {
			attachedImages = append(attachedImages, attachedImageToInfo(*dirImage))
		}
	} else {
		for _, image := range srcImages {
			attachedImages = append(attachedImages, attachedImageToInfo(image))
		}
	}

	result := Result{
		Name:            filepath.Base(fsPath),
		Duration:        float64(src.Duration()) / float64(time.Second),
		ReplayGainTrack: src.ReplayGain(aurelib.ReplayGainTrack, true),
		ReplayGainAlbum: src.ReplayGain(aurelib.ReplayGainAlbum, true),
		Tags:            maputil.LowerCaseKeys(src.Tags()),
		AttachedImages:  attachedImages,
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

func (ml *Library) handleTrackImageRequest(
	libraryPath string,
	indexStr string,
	w http.ResponseWriter,
	req *http.Request,
) {
	fsPath := ml.libraryToFsPath(libraryPath)
	if info, err := os.Stat(fsPath); err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, req)
		return
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 {
		http.NotFound(w, req)
		return
	}

	src, err := newAudioSource(fsPath)
	if err != nil {
		log.Printf("failed to open source '%v': %v\n", fsPath, err)
		http.NotFound(w, req)
		return
	}
	defer src.Destroy()

	images := src.AttachedImages()
	if len(images) == 0 {
		if dirImage := findDirectoryImage(fsPath); dirImage != nil {
			images = []aurelib.AttachedImage{*dirImage}
		}
	}

	if index >= len(images) {
		http.NotFound(w, req)
		return
	}
	image := &images[index]

	// Set appropriate MIME type
	var mimeType string
	switch image.Format {
	case aurelib.AttachedImageJPEG:
		mimeType = "image/jpeg"
	case aurelib.AttachedImagePNG:
		mimeType = "image/png"
	case aurelib.AttachedImageGIF:
		mimeType = "image/gif"
	default:
		http.NotFound(w, req)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set(
		"Cache-Control",
		fmt.Sprintf("public, max-age=%v", int(cachedImageMaxAge/time.Second)))
	if _, err := w.Write(image.Data); err != nil {
		log.Printf("failed to write image response: %v\n", err)
	}
}
