package media

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/beakbeak/aurelius/internal/mediadb"
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
	if err := ml.db.SetFavorite(libraryPath, favorite); err != nil {
		slog.ErrorContext(ctx, "SetFavorite failed", "value", favorite, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		writeJson(ctx, w, nil)
	}
}

type attachedImageInfo struct {
	MimeType string `json:"mimeType"`
	Size     int    `json:"size"`
	Url      string `json:"url"`
}

type trackInfoResult struct {
	Name            string              `json:"name"`
	Url             string              `json:"url"`
	Duration        float64             `json:"duration"`
	ReplayGainTrack float64             `json:"replayGainTrack"`
	ReplayGainAlbum float64             `json:"replayGainAlbum"`
	Favorite        bool                `json:"favorite"`
	Tags            map[string]string   `json:"tags"`
	AttachedImages  []attachedImageInfo `json:"attachedImages"`
	Codec           string              `json:"codec"`
	BitRate         int                 `json:"bitRate"`
	SampleRate      uint                `json:"sampleRate"`
	SampleFormat    string              `json:"sampleFormat"`
	Dir             string              `json:"dir"`
}

func (ml *Library) buildTrackInfo(track *mediadb.Track, favorite bool) trackInfoResult {
	images := make([]attachedImageInfo, 0, len(track.Images))
	for _, img := range track.Images {
		images = append(images, attachedImageInfo{
			MimeType: img.MimeType,
			Size:     img.Size,
			Url:      ml.makeImageUrl(img.Hash),
		})
	}

	replayGainTrack := 1.0
	replayGainAlbum := 1.0
	if track.Metadata.ReplayGain != nil {
		replayGainTrack = track.Metadata.ReplayGain.Track
		replayGainAlbum = track.Metadata.ReplayGain.Album
	}

	trackPath := joinLibraryPath(track.Dir, track.Name)
	return trackInfoResult{
		Name:            track.Name,
		Url:             ml.libraryToUrlPath("tracks", trackPath),
		Duration:        track.Metadata.Duration,
		ReplayGainTrack: replayGainTrack,
		ReplayGainAlbum: replayGainAlbum,
		Favorite:        favorite,
		Tags:            track.Tags,
		AttachedImages:  images,
		Codec:           track.Metadata.Codec,
		BitRate:         track.Metadata.BitRate,
		SampleRate:      track.Metadata.SampleRate,
		SampleFormat:    track.Metadata.SampleFormat,
		Dir:             ml.libraryToUrlPath("dirs", track.Dir),
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

	favorite, err := ml.db.IsFavorite(libraryPath)
	if err != nil {
		slog.ErrorContext(ctx, "IsFavorite failed", "error", err)
	}

	writeJson(ctx, w, ml.buildTrackInfo(track, favorite))
}

func (ml *Library) handleImageByHash(
	hashHex string,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx := req.Context()

	hash, err := hex.DecodeString(hashHex)
	if err != nil {
		http.NotFound(w, req)
		return
	}

	data, mimeType, err := ml.db.GetImageDataByHash(hash)
	if err != nil {
		slog.ErrorContext(ctx, "GetImageDataByHash failed", "error", err)
		http.NotFound(w, req)
		return
	}
	if data == nil {
		http.NotFound(w, req)
		return
	}

	w.Header().Set("Cache-Control", "max-age=31536000, immutable")
	w.Header().Set("Content-Type", mimeType)
	if _, err := w.Write(data); err != nil {
		slog.ErrorContext(ctx, "failed to write image response", "error", err)
	}
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
