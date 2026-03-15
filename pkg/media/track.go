package media

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

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

func (ml *Library) handleTrackFavorite(
	libraryPath string,
	favorite bool,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx := req.Context()
	fsPath := ml.libraryToFsPath(libraryPath)
	if info, err := os.Stat(fsPath); err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, req)
		return
	}
	if err := ml.setFavorite(libraryPath, favorite); err != nil {
		slog.ErrorContext(ctx, "setFavorite failed", "value", favorite, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		writeJson(ctx, w, nil)
	}
}

func (ml *Library) handleTrackInfo(
	libraryPath string,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx := req.Context()

	dir, name := path.Split(libraryPath)
	dir = cleanLibraryPath(dir)

	track, err := ml.db.GetTrack(dir, name)
	if err != nil {
		slog.ErrorContext(ctx, "GetTrack failed", "error", err)
	}

	if track == nil {
		http.NotFound(w, req)
		return
	}

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
		Dir             string              `json:"dir"`
	}

	// Build attached image list from DB data + directory images.
	images := make([]AttachedImageInfo, 0, len(track.AttachedImages))
	for _, img := range track.AttachedImages {
		images = append(images, AttachedImageInfo{
			MimeType: img.MimeType,
			Size:     img.Size,
		})
	}

	// Add directory images (still discovered at request time).
	fsPath := ml.libraryToFsPath(libraryPath)
	for _, dirImg := range getDirectoryImages(fsPath) {
		images = append(images, AttachedImageInfo{
			MimeType: dirImg.format.MimeType(),
			Size:     int(dirImg.size),
		})
	}

	replayGainTrack := 1.0
	replayGainAlbum := 1.0
	if track.Metadata.ReplayGain != nil {
		replayGainTrack = track.Metadata.ReplayGain.Track
		replayGainAlbum = track.Metadata.ReplayGain.Album
	}

	result := Result{
		Name:            track.Name,
		Duration:        track.Metadata.Duration,
		ReplayGainTrack: replayGainTrack,
		ReplayGainAlbum: replayGainAlbum,
		Tags:            track.Tags,
		AttachedImages:  images,
		BitRate:         track.Metadata.BitRate,
		SampleRate:      track.Metadata.SampleRate,
		SampleFormat:    track.Metadata.SampleFormat,
		Dir:             ml.libraryToUrlPath("dirs", cleanLibraryPath(dir)),
	}

	if favorite, err := ml.isFavorite(libraryPath); err != nil {
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
	return ml.db.IsFavorite(path)
}

func (ml *Library) setFavorite(path string, favorite bool) error {
	return ml.db.SetFavorite(path, favorite)
}

func (ml *Library) handleTrackImage(
	libraryPath string,
	indexStr string,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx := req.Context()
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
