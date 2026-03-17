package media

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
)

func (ml *Library) handleTrackFavorite(
	libraryPath string,
	favorite bool,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx := req.Context()

	track, err := ml.db.GetTrack(libraryPath)
	if err != nil {
		slog.ErrorContext(ctx, "GetTrack failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if track == nil {
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

	track, err := ml.db.GetTrack(libraryPath)
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

	images := make([]AttachedImageInfo, 0, len(track.Images))
	for _, img := range track.Images {
		images = append(images, AttachedImageInfo{
			MimeType: img.MimeType,
			Size:     img.Size,
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
		Dir:             ml.libraryToUrlPath("dirs", track.Dir),
	}

	if favorite, err := ml.isFavorite(libraryPath); err != nil {
		slog.ErrorContext(ctx, "isFavorite failed", "error", err)
	} else {
		result.Favorite = favorite
	}

	writeJson(ctx, w, result)
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

	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 {
		http.NotFound(w, req)
		return
	}

	data, mimeType, hash, err := ml.db.GetTrackImageData(libraryPath, index)
	if err != nil {
		slog.ErrorContext(ctx, "GetTrackImageData failed", "error", err)
		http.NotFound(w, req)
		return
	}
	if data == nil {
		http.NotFound(w, req)
		return
	}

	etag := fmt.Sprintf("\"%s\"", hex.EncodeToString(hash))
	w.Header().Set("ETag", etag)
	if match := req.Header.Get("If-None-Match"); match != "" {
		if match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-cache")

	w.Header().Set("Content-Type", mimeType)
	if _, err := w.Write(data); err != nil {
		slog.ErrorContext(ctx, "failed to write image response", "error", err)
	}
}
