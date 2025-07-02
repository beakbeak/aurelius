package media

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/beakbeak/aurelius/internal/maputil"
	"github.com/beakbeak/aurelius/pkg/aurelib"
	"github.com/beakbeak/aurelius/pkg/fragment"
)

const (
	cachedImageMaxAge     = 1 * time.Hour
	maxDirectoryImageSize = 2 * 1024 * 1024
)

var (
	coverImageRegex = regexp.MustCompile(`[Ff][Rr][Oo][Nn][Tt]|[Cc][Oo][Vv][Ee][Rr]|[Tt][Hh][Uu][Mm][Bb]|F$`)
)

type directoryImage struct {
	name   string
	path   string
	size   int64
	format aurelib.AttachedImageFormat
}

type attachedOrDirectoryImage struct {
	attached *aurelib.AttachedImage
	dir      *directoryImage
}

func (img *attachedOrDirectoryImage) Size() int {
	if img.attached != nil {
		return len(img.attached.Data)
	}
	return int(img.dir.size)
}

func (img *attachedOrDirectoryImage) Format() aurelib.AttachedImageFormat {
	if img.attached != nil {
		return img.attached.Format
	}
	return img.dir.format
}

func (img *attachedOrDirectoryImage) ToAttachedImage() (*aurelib.AttachedImage, error) {
	if img.attached != nil {
		return img.attached, nil
	}

	data, err := os.ReadFile(img.dir.path)
	if err != nil {
		return nil, err
	}

	return &aurelib.AttachedImage{
		Data:   data,
		Format: img.dir.format,
	}, nil
}

func getDirectoryImages(trackPath string) []directoryImage {
	dir := filepath.Dir(trackPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	knownImageExts := []string{".jpg", ".jpeg", ".png", ".gif"}
	var images []directoryImage

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !slices.Contains(knownImageExts, ext) {
			continue
		}
		imagePath := filepath.Join(dir, entry.Name())
		info, err := os.Stat(imagePath)
		if err != nil {
			continue
		}
		if info.Size() > maxDirectoryImageSize {
			slog.Info(
				"skipping oversized directory image",
				"path", imagePath, "size", info.Size(),
				"maxSize", maxDirectoryImageSize)
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
		images = append(images, directoryImage{
			name:   entry.Name(),
			path:   imagePath,
			size:   info.Size(),
			format: format,
		})
	}

	// Sort cover images first, then lexicographically
	slices.SortFunc(images, func(a, b directoryImage) int {
		aNameWithoutExt := strings.TrimSuffix(a.name, filepath.Ext(a.name))
		bNameWithoutExt := strings.TrimSuffix(b.name, filepath.Ext(b.name))

		aCover := coverImageRegex.MatchString(aNameWithoutExt)
		bCover := coverImageRegex.MatchString(bNameWithoutExt)

		if aCover && !bCover {
			return -1
		}
		if !aCover && bCover {
			return 1
		}
		return strings.Compare(a.name, b.name)
	})

	return images
}

func getAttachedAndDirectoryImages(src aurelib.Source, fsPath string) []attachedOrDirectoryImage {
	attachedImages := src.AttachedImages()
	directoryImages := getDirectoryImages(fsPath)
	result := make([]attachedOrDirectoryImage, 0, len(attachedImages)+len(directoryImages))
	for i := range attachedImages {
		result = append(result, attachedOrDirectoryImage{
			attached: &attachedImages[i],
		})
	}
	for i := range directoryImages {
		result = append(result, attachedOrDirectoryImage{
			dir: &directoryImages[i],
		})
	}
	return result
}

func (ml *Library) handleTrackRequest(
	ctx context.Context,
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
			slog.InfoContext(ctx, "stream", "path", libraryPath)
			ml.handleTrackStreamRequest(ctx, fsPath, w, req)
		case "info":
			ml.handleTrackInfoRequest(ctx, libraryPath, fsPath, w, req)
		default:
			handled = false
		}

	case http.MethodPost:
		switch resource {
		case "favorite":
			if err := ml.setFavorite(libraryPath, true); err != nil {
				slog.ErrorContext(ctx, "Favorite failed", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				writeJson(ctx, w, nil)
			}

		case "unfavorite":
			if err := ml.setFavorite(libraryPath, false); err != nil {
				slog.ErrorContext(ctx, "Unfavorite failed", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				writeJson(ctx, w, nil)
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
	ctx context.Context,
	urlPath string,
	fsPath string,
	w http.ResponseWriter,
	req *http.Request,
) {
	src, err := newAudioSource(fsPath)
	if err != nil {
		slog.ErrorContext(ctx, "failed to open source", "path", fsPath, "error", err)
		http.NotFound(w, req)
		return
	}
	defer src.Destroy()

	type AttachedImageInfo struct {
		MimeType string `json:"mimeType"`
		Size     int    `json:"size"`
	}

	type Result struct {
		Name            string              `json:"name"`
		Duration        float64             `json:"duration"`
		ReplayGainTrack float64             `json:"replayGainTrack"`
		ReplayGainAlbum float64             `json:"replayGainAlbum"`
		Favorite        bool                `json:"favorite"`
		Tags            map[string]string   `json:"tags"`
		AttachedImages  []AttachedImageInfo `json:"attachedImages"`
		BitRate         int                 `json:"bitRate"`
		SampleRate      uint                `json:"sampleRate"`
		SampleFormat    string              `json:"sampleFormat"`
	}

	images := getAttachedAndDirectoryImages(src, fsPath)
	attachedImages := make([]AttachedImageInfo, 0, len(images))
	for _, image := range images {
		attachedImages = append(attachedImages, AttachedImageInfo{
			MimeType: image.Format().MimeType(),
			Size:     image.Size(),
		})
	}

	streamInfo := src.StreamInfo()
	result := Result{
		Name:            filepath.Base(fsPath),
		Duration:        float64(src.Duration()) / float64(time.Second),
		ReplayGainTrack: src.ReplayGain(aurelib.ReplayGainTrack, true),
		ReplayGainAlbum: src.ReplayGain(aurelib.ReplayGainAlbum, true),
		Tags:            maputil.LowerCaseKeys(src.Tags()),
		AttachedImages:  attachedImages,
		BitRate:         src.BitRate(),
		SampleRate:      streamInfo.SampleRate,
		SampleFormat:    streamInfo.SampleFormat(),
	}

	if favorite, err := ml.isFavorite(urlPath); err != nil {
		slog.ErrorContext(ctx, "isFavorite failed", "error", err)
	} else {
		result.Favorite = favorite
	}

	writeJson(ctx, w, result)
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
	ctx context.Context,
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
		slog.ErrorContext(ctx, "failed to open source", "path", fsPath, "error", err)
		http.NotFound(w, req)
		return
	}
	defer src.Destroy()

	images := getAttachedAndDirectoryImages(src, fsPath)
	if index >= len(images) {
		http.NotFound(w, req)
		return
	}

	image, err := images[index].ToAttachedImage()
	if err != nil {
		slog.ErrorContext(ctx, "failed to load image", "error", err)
		http.NotFound(w, req)
		return
	}

	etag := fmt.Sprintf("\"%x\"", len(image.Data))
	w.Header().Set("ETag", etag)
	if match := req.Header.Get("If-None-Match"); match != "" {
		if match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set(
		"Cache-Control",
		fmt.Sprintf("max-age=%v", int(cachedImageMaxAge/time.Second)))

	w.Header().Set("Content-Type", image.Format.MimeType())
	if _, err := w.Write(image.Data); err != nil {
		slog.ErrorContext(ctx, "failed to write image response", "error", err)
	}
}
